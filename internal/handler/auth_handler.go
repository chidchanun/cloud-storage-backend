package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"cloud-storage-backend/internal/auth"
	"cloud-storage-backend/internal/email"
	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"

	"golang.org/x/crypto/bcrypt"
)

// AuthHandler ใช้จัดการ request ที่เกี่ยวกับ authentication
const maxProfilePictureSize int64 = 5 << 20 // 5 MB

type AuthHandler struct {
	userRepo              *repository.UserRepository
	userTokenRepo         *repository.UserTokenRepository
	emailVerificationRepo *repository.EmailVerificationRepository

	googleAuthRepo     *repository.GoogleAuthRepository
	googleOAuthService *auth.GoogleOAuthService
	emailSender        *email.SMTPSender

	jwtService    *auth.JWTService
	cookieService *auth.CookieService

	uploadRoot    string
	maxUploadSize int64

	frontendAuthSuccessURL string
	frontendAuthErrorURL   string

	emailVerificationURL          string
	frontendEmailVerifySuccessURL string
	frontendEmailVerifyErrorURL   string
}

// NewAuthHandler สร้าง AuthHandler
func NewAuthHandler(
	userRepo *repository.UserRepository,
	userTokenRepo *repository.UserTokenRepository,
	emailVerificationRepo *repository.EmailVerificationRepository,
	jwtService *auth.JWTService,
	cookieService *auth.CookieService,

	googleAuthRepo *repository.GoogleAuthRepository,
	googleOAuthService *auth.GoogleOAuthService,
	emailSender *email.SMTPSender,

	uploadRoot string,
	maxUploadSize int64,

	frontendAuthSuccessURL string,
	frontendAuthErrorURL string,
	emailVerificationURL string,
	frontendEmailVerifySuccessURL string,
	frontendEmailVerifyErrorURL string,
) *AuthHandler {
	if uploadRoot == "" {
		uploadRoot = "uploads"
	}

	if maxUploadSize <= 0 {
		maxUploadSize = maxProfilePictureSize
	}

	return &AuthHandler{
		userRepo:              userRepo,
		userTokenRepo:         userTokenRepo,
		emailVerificationRepo: emailVerificationRepo,

		googleAuthRepo:     googleAuthRepo,
		googleOAuthService: googleOAuthService,
		emailSender:        emailSender,

		jwtService:    jwtService,
		cookieService: cookieService,

		uploadRoot:    uploadRoot,
		maxUploadSize: maxUploadSize,

		frontendAuthSuccessURL: frontendAuthSuccessURL,
		frontendAuthErrorURL:   frontendAuthErrorURL,

		emailVerificationURL:          emailVerificationURL,
		frontendEmailVerifySuccessURL: frontendEmailVerifySuccessURL,
		frontendEmailVerifyErrorURL:   frontendEmailVerifyErrorURL,
	}
}

// GoogleLogin เริ่มกระบวนการเข้าสู่ระบบด้วย Google
// GET /api/auth/google
func (h *AuthHandler) GoogleLogin(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.googleOAuthService == nil ||
		!h.googleOAuthService.IsConfigured() {
		response.Error(
			w,
			http.StatusServiceUnavailable,
			"ระบบเข้าสู่ระบบด้วย Google ยังไม่ได้ตั้งค่า",
		)
		return
	}

	// สร้าง state แบบสุ่มเพื่อป้องกัน OAuth CSRF
	state, err := h.googleOAuthService.GenerateState()
	if err != nil {
		log.Printf(
			"generate Google OAuth state error: %v",
			err,
		)

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถเริ่มเข้าสู่ระบบด้วย Google ได้",
		)
		return
	}

	// เก็บ state ใน HttpOnly Cookie
	h.googleOAuthService.SetStateCookie(
		w,
		state,
	)

	// สร้าง Google authorization URL
	authorizationURL :=
		h.googleOAuthService.AuthorizationURL(state)

	// ส่ง browser ไปหน้า Google
	http.Redirect(
		w,
		r,
		authorizationURL,
		http.StatusSeeOther,
	)
}

// GoogleCallback รับผลลัพธ์จาก Google
// GET /api/auth/google/callback
func (h *AuthHandler) GoogleCallback(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.googleOAuthService == nil ||
		!h.googleOAuthService.IsConfigured() {
		h.redirectGoogleAuthError(
			w,
			r,
			"google_not_configured",
		)
		return
	}

	// ลบ state cookie หลัง callback เสมอ
	// เพื่อไม่ให้ state เดิมถูกนำกลับมาใช้ซ้ำ
	defer h.googleOAuthService.ClearStateCookie(w)

	// Google อาจส่ง error กลับมา เช่น user กดยกเลิก
	googleError := strings.TrimSpace(
		r.URL.Query().Get("error"),
	)

	if googleError != "" {
		log.Printf(
			"Google OAuth callback error: %s",
			googleError,
		)

		h.redirectGoogleAuthError(
			w,
			r,
			"google_login_cancelled",
		)
		return
	}

	callbackState := strings.TrimSpace(
		r.URL.Query().Get("state"),
	)

	// ตรวจว่า state ตรงกับ Cookie ที่เราสร้างไว้
	if err := h.googleOAuthService.ValidateState(
		r,
		callbackState,
	); err != nil {
		log.Printf(
			"validate Google OAuth state error: %v",
			err,
		)

		h.redirectGoogleAuthError(
			w,
			r,
			"invalid_oauth_state",
		)
		return
	}

	authorizationCode := strings.TrimSpace(
		r.URL.Query().Get("code"),
	)

	if authorizationCode == "" {
		h.redirectGoogleAuthError(
			w,
			r,
			"missing_authorization_code",
		)
		return
	}

	// แลก authorization code และตรวจ Google ID Token
	identity, err := h.googleOAuthService.ExchangeCode(
		r.Context(),
		authorizationCode,
	)
	if err != nil {
		log.Printf(
			"exchange Google authorization code error: %v",
			err,
		)

		h.redirectGoogleAuthError(
			w,
			r,
			"invalid_google_token",
		)
		return
	}

	// หา user เดิม หรือสร้าง user ใหม่
	user, err := h.googleAuthRepo.FindOrCreateGoogleUser(
		r.Context(),
		identity,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			repository.ErrGoogleEmailNotVerified,
		):
			h.redirectGoogleAuthError(
				w,
				r,
				"google_email_not_verified",
			)

		case errors.Is(
			err,
			repository.ErrGoogleAccountConflict,
		):
			h.redirectGoogleAuthError(
				w,
				r,
				"google_account_conflict",
			)

		case errors.Is(
			err,
			repository.ErrInvalidGoogleIdentity,
		):
			h.redirectGoogleAuthError(
				w,
				r,
				"invalid_google_identity",
			)

		default:
			log.Printf(
				"find or create Google user error: %v",
				err,
			)

			h.redirectGoogleAuthError(
				w,
				r,
				"google_login_failed",
			)
		}

		return
	}

	// ออก JWT ของระบบเราเอง
	if err := h.createAuthenticatedSession(
		r,
		w,
		user,
	); err != nil {
		log.Printf(
			"create Google authenticated session error: %v",
			err,
		)

		h.redirectGoogleAuthError(
			w,
			r,
			"session_creation_failed",
		)
		return
	}

	// Login สำเร็จ กลับไป Angular
	http.Redirect(
		w,
		r,
		h.frontendAuthSuccessURL,
		http.StatusSeeOther,
	)
}

// Register คือ handler สำหรับ POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)

	if req.FirstName == "" || req.LastName == "" || req.Email == "" || req.Phone == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "all fields are required")
		return
	}

	if len(req.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// เช็ก email ซ้ำก่อน
	_, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err == nil {
		response.Error(w, http.StatusConflict, "email already exists")
		return
	}

	parsedEmail, err := mail.ParseAddress(req.Email)
	if err != nil || parsedEmail.Address != req.Email {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid email format",
		)
		return
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		response.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// hash password ก่อนบันทึกลง database
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot hash password")
		return
	}

	passwordHashString := string(passwordHash)

	user := &models.User{
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        req.Email,
		Phone:        &req.Phone,
		PasswordHash: &passwordHashString,
	}
	if err := h.userRepo.Create(r.Context(), user); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot create user",
		)
		return
	}

	if err := h.sendEmailVerification(r, user); err != nil {
		log.Printf(
			"send verification email error: %v",
			err,
		)

		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot send verification email",
		)
		return
	}

	accessToken, expiredAt, err := h.jwtService.Generate(user)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot generate token")
		return
	}

	tokenHash := auth.HashToken(accessToken)

	userToken := &models.UserToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: expiredAt,
	}

	if err := h.userTokenRepo.Upsert(r.Context(), userToken); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot save user session",
		)
		return
	}

	h.cookieService.SetAccessToken(w, accessToken, expiredAt)

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": "register successful, please verify your email",
		"user":    models.NewUserResponse(user),
	})
}

// VerifyEmail ใช้ยืนยันอีเมลจากลิงก์ที่ส่งไปใน inbox
// GET /api/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rawToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawToken == "" {
		h.redirectEmailVerification(
			w,
			r,
			h.frontendEmailVerifyErrorURL,
		)
		return
	}

	tokenHash := auth.HashToken(rawToken)

	verificationToken, err := h.emailVerificationRepo.FindValidByTokenHash(
		r.Context(),
		tokenHash,
	)
	if err != nil {
		h.redirectEmailVerification(
			w,
			r,
			h.frontendEmailVerifyErrorURL,
		)
		return
	}

	if err := h.userRepo.MarkEmailVerified(
		r.Context(),
		verificationToken.UserID,
	); err != nil {
		log.Printf(
			"mark email verified error: %v",
			err,
		)

		h.redirectEmailVerification(
			w,
			r,
			h.frontendEmailVerifyErrorURL,
		)
		return
	}

	if err := h.emailVerificationRepo.MarkUsed(
		r.Context(),
		verificationToken.ID,
	); err != nil {
		log.Printf(
			"mark verification token used error: %v",
			err,
		)
	}

	user, err := h.userRepo.FindByID(
		r.Context(),
		verificationToken.UserID,
	)
	if err != nil {
		log.Printf(
			"load verified user error: %v",
			err,
		)

		h.redirectEmailVerification(
			w,
			r,
			h.frontendEmailVerifyErrorURL,
		)
		return
	}

	if err := h.createAuthenticatedSession(
		r,
		w,
		user,
	); err != nil {
		log.Printf(
			"create verified email session error: %v",
			err,
		)

		h.redirectEmailVerification(
			w,
			r,
			h.frontendEmailVerifyErrorURL,
		)
		return
	}

	h.redirectEmailVerification(
		w,
		r,
		h.frontendEmailVerifySuccessURL,
	)
}

// Login คือ handler สำหรับ POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		response.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	if user.PasswordHash == nil {
		response.Error(
			w,
			http.StatusUnauthorized,
			"บัญชีนี้เข้าสู่ระบบด้วย Google",
		)
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(*user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		response.Error(
			w,
			http.StatusUnauthorized,
			"อีเมลหรือรหัสผ่านไม่ถูกต้อง",
		)
		return
	}

	accessToken, expiredAt, err := h.jwtService.Generate(user)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot generate token",
		)
		return
	}

	tokenHash := auth.HashToken(accessToken)

	userToken := &models.UserToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: expiredAt,
	}

	if err := h.userTokenRepo.Upsert(r.Context(), userToken); err != nil {

		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot save user session",
		)
		return
	}
	h.cookieService.SetAccessToken(w, accessToken, expiredAt)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "login successful",
		"user":    models.NewUserResponse(user),
	})

}

// Me คือ handler สำหรับ GET /api/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	response.JSON(w, http.StatusOK, models.NewUserResponse(user))
}

func (h *AuthHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	if strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.ConfirmPassword) == "" {
		response.Error(w, http.StatusBadRequest, "password and confirm password are required")
		return
	}

	if req.Password != req.ConfirmPassword {
		response.Error(w, http.StatusBadRequest, "password confirmation does not match")
		return
	}

	if message := validatePasswordPolicy(req.Password); message != "" {
		response.Error(w, http.StatusBadRequest, message)
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	if user.PasswordHash != nil {
		response.Error(w, http.StatusConflict, "this account already has a password")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot hash password")
		return
	}

	updatedUser, err := h.userRepo.SetPasswordHash(
		r.Context(),
		claims.UserID,
		string(passwordHash),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusConflict, "this account already has a password")
			return
		}

		response.Error(w, http.StatusInternalServerError, "cannot set password")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "password has been set",
		"user":    models.NewUserResponse(updatedUser),
	})
}

func (h *AuthHandler) UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		h.maxUploadSize+multipartOverhead,
	)

	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		response.Error(w, http.StatusRequestEntityTooLarge, "ขนาดรูปโปรไฟล์ใหญ่เกิน")
		return
	}

	uploadFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			response.Error(w, http.StatusBadRequest, "file is required")
			return
		}

		response.Error(w, http.StatusBadRequest, "cannot read uploaded file")
		return
	}
	defer uploadFile.Close()

	originalName := cleanOriginalName(fileHeader.Filename)
	if originalName == "" {
		response.Error(w, http.StatusBadRequest, "invalid file name")
		return
	}

	if fileHeader.Size > h.maxUploadSize {
		response.Error(w, http.StatusRequestEntityTooLarge, "profile picture is too large")
		return
	}

	headerBuffer := make([]byte, 512)

	bytesRead, err := uploadFile.Read(headerBuffer)
	if err != nil && !errors.Is(err, io.EOF) {
		response.Error(w, http.StatusBadRequest, "cannot inspect uploaded file")
		return
	}

	mimeType := http.DetectContentType(headerBuffer[:bytesRead])
	if !isAllowedProfilePictureMime(mimeType) {
		response.Error(w, http.StatusBadRequest, "profile picture must be jpeg, png, gif, or webp")
		return
	}

	if _, err := uploadFile.Seek(0, io.SeekStart); err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot reset uploaded file")
		return
	}

	storedName, err := generateStoredName(originalName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot generate file name")
		return
	}

	userID := strconv.Itoa(claims.UserID)
	userDirectory := filepath.Join(h.uploadRoot, "profiles", userID)

	if err := os.MkdirAll(userDirectory, 0750); err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create profile directory")
		return
	}

	storagePath := filepath.Join(userDirectory, storedName)

	destination, err := os.OpenFile(
		storagePath,
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0640,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create profile picture")
		return
	}

	writtenBytes, copyErr := io.Copy(
		destination,
		io.LimitReader(uploadFile, h.maxUploadSize+1),
	)

	closeErr := destination.Close()

	if copyErr != nil {
		_ = os.Remove(storagePath)
		response.Error(w, http.StatusInternalServerError, "cannot store profile picture")
		return
	}

	if closeErr != nil {
		_ = os.Remove(storagePath)
		response.Error(w, http.StatusInternalServerError, "cannot close profile picture")
		return
	}

	if writtenBytes > h.maxUploadSize {
		_ = os.Remove(storagePath)
		response.Error(w, http.StatusRequestEntityTooLarge, "profile picture is too large")
		return
	}

	oldUser, _ := h.userRepo.FindByID(r.Context(), claims.UserID)
	publicPath := "/uploads/profiles/" + userID + "/" + storedName

	user, err := h.userRepo.UpdatePicturePath(r.Context(), claims.UserID, publicPath)
	if err != nil {
		_ = os.Remove(storagePath)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "cannot update profile picture")
		return
	}

	if oldUser != nil && oldUser.PicturePath != nil && *oldUser.PicturePath != publicPath {
		_ = removeLocalProfilePicture(h.uploadRoot, *oldUser.PicturePath)
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "profile picture uploaded",
		"user":    models.NewUserResponse(user),
	})
}

// Logout คือ handler สำหรับ POST /api/auth/logout
func (h *AuthHandler) Logout(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	// อ่าน access token จาก HttpOnly Cookie
	accessToken, err := h.cookieService.GetAccessToken(r)

	if err == nil && accessToken != "" {
		// แปลง token จริงเป็น hash ให้ตรงกับข้อมูลใน database
		tokenHash := auth.HashToken(accessToken)

		// ยกเลิก session ในฐานข้อมูล
		if err := h.userTokenRepo.RevokeByTokenHash(
			r.Context(),
			tokenHash,
		); err != nil {
			// ลบ Cookie แม้ฐานข้อมูลมีปัญหา
			h.cookieService.ClearAccessToken(w)

			response.Error(
				w,
				http.StatusInternalServerError,
				"cannot revoke user session",
			)
			return
		}
	}

	// ลบ access_token ออกจาก Browser
	h.cookieService.ClearAccessToken(w)

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "logout successful",
	})
}

func isAllowedProfilePictureMime(mimeType string) bool {
	switch mimeType {
	case "image/jpeg",
		"image/png",
		"image/gif",
		"image/webp":
		return true
	default:
		return false
	}
}

func removeLocalProfilePicture(uploadRoot string, publicPath string) error {
	if !strings.HasPrefix(publicPath, "/uploads/profiles/") {
		return nil
	}

	relativePath := strings.TrimPrefix(publicPath, "/uploads/")
	localPath := filepath.Join(uploadRoot, filepath.FromSlash(relativePath))

	absoluteUploadRoot, err := filepath.Abs(uploadRoot)
	if err != nil {
		return err
	}

	absoluteLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absoluteLocalPath, absoluteUploadRoot+string(os.PathSeparator)) {
		return nil
	}

	return os.Remove(absoluteLocalPath)
}

func (h *AuthHandler) sendEmailVerification(
	r *http.Request,
	user *models.User,
) error {
	if h.emailVerificationRepo == nil || user == nil || user.ID <= 0 {
		return nil
	}

	rawToken, err := auth.GenerateVerificationToken()
	if err != nil {
		return err
	}

	verificationToken := &models.EmailVerificationToken{
		UserID:    user.ID,
		TokenHash: auth.HashToken(rawToken),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	if err := h.emailVerificationRepo.Create(
		r.Context(),
		verificationToken,
	); err != nil {
		return err
	}

	verificationLink, err := h.buildEmailVerificationLink(rawToken)
	if err != nil {
		return err
	}

	fullName := strings.TrimSpace(user.FirstName + " " + user.LastName)

	if h.emailSender == nil || !h.emailSender.IsConfigured() {
		log.Printf(
			"email verification link for %s: %s",
			user.Email,
			verificationLink,
		)
		return nil
	}

	return h.emailSender.SendVerificationEmail(
		user.Email,
		fullName,
		verificationLink,
	)
}

func (h *AuthHandler) buildEmailVerificationLink(
	rawToken string,
) (string, error) {
	baseURL := strings.TrimSpace(h.emailVerificationURL)
	if baseURL == "" {
		baseURL = "http://localhost:8080/api/auth/verify-email"
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()
	query.Set("token", rawToken)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func (h *AuthHandler) redirectEmailVerification(
	w http.ResponseWriter,
	r *http.Request,
	redirectURL string,
) {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "email verification completed",
		})
		return
	}

	http.Redirect(
		w,
		r,
		redirectURL,
		http.StatusSeeOther,
	)
}

// createAuthenticatedSession สร้าง JWT,
// บันทึก token hash และตั้ง HttpOnly Cookie
//
// ใช้ร่วมกันได้ทั้ง Local Login และ Google Login
func (h *AuthHandler) createAuthenticatedSession(
	r *http.Request,
	w http.ResponseWriter,
	user *models.User,
) error {
	if user == nil || user.ID <= 0 {
		return errors.New("invalid user for session")
	}

	// สร้าง JWT พร้อมวันหมดอายุ
	token, expiresAt, err := h.jwtService.Generate(user)
	if err != nil {
		return err
	}

	// ไม่เก็บ JWT จริงลงฐานข้อมูล
	// เก็บเฉพาะ SHA-256 hash
	tokenHash := auth.HashToken(token)

	// บันทึกหรืออัปเดต session ของ user
	userToken := &models.UserToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: expiresAt,
	}

	if err := h.userTokenRepo.Upsert(r.Context(), userToken); err != nil {
		return err
	}

	// ส่ง JWT ให้ Browser ผ่าน HttpOnly Cookie
	h.cookieService.SetAccessToken(
		w,
		token,
		expiresAt,
	)

	return nil
}

// redirectGoogleAuthError ส่งผู้ใช้กลับหน้า Login ของ Frontend
// พร้อม error code ที่ปลอดภัยสำหรับแสดงผล
func (h *AuthHandler) redirectGoogleAuthError(
	w http.ResponseWriter,
	r *http.Request,
	errorCode string,
) {
	redirectURL := strings.TrimSpace(
		h.frontendAuthErrorURL,
	)

	if redirectURL == "" {
		response.Error(
			w,
			http.StatusUnauthorized,
			"ไม่สามารถเข้าสู่ระบบด้วย Google ได้",
		)
		return
	}

	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		log.Printf(
			"parse frontend auth error URL: %v",
			err,
		)

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถเข้าสู่ระบบด้วย Google ได้",
		)
		return
	}

	query := parsedURL.Query()
	query.Set("error", errorCode)

	parsedURL.RawQuery = query.Encode()

	http.Redirect(
		w,
		r,
		parsedURL.String(),
		http.StatusSeeOther,
	)
}

func validatePasswordPolicy(password string) string {
	if len(password) < 8 || len(password) > 64 {
		return "password must be 8-64 characters"
	}

	var hasLower bool
	var hasUpper bool
	var hasDigit bool
	var hasSpecial bool

	for _, char := range password {
		if unicode.IsSpace(char) {
			return "password must not contain spaces"
		}

		switch {
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsDigit(char):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
		return "password must include lowercase, uppercase, number, and special character"
	}

	return ""
}
