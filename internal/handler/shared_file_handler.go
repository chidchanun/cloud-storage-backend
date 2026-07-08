package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud-storage-backend/internal/auth"
	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

// SharedFileHandler ใช้จัดการ API ที่เกี่ยวข้องกับแชร์ไฟล์
type SharedFileHandler struct {
	sharedFileRepo     *repository.SharedFileRepository
	publicFileLinkRepo *repository.PublicFileLinkRepository
	fileRepo           *repository.FileRepository
	userRepo           *repository.UserRepository
	uploadRoot         string
}

// NewSharedFileHandler สร้าง SharedFileHandler
func NewSharedFileHandler(
	sharedFileRepo *repository.SharedFileRepository,
	publicFileLinkRepo *repository.PublicFileLinkRepository,
	fileRepo *repository.FileRepository,
	userRepo *repository.UserRepository,
	uploadRoot string,
) *SharedFileHandler {
	return &SharedFileHandler{
		sharedFileRepo:     sharedFileRepo,
		publicFileLinkRepo: publicFileLinkRepo,
		fileRepo:           fileRepo,
		userRepo:           userRepo,
		uploadRoot:         uploadRoot,
	}
}

// Create สร้างการแชร์ไฟล์
// POST /api/files/share-file
func (h *SharedFileHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)

		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)

		return
	}

	// จำกัดขนาด JSON body
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		64*1024,
	)

	var req models.SharedFileAddUserRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	req.Permission = strings.TrimSpace(strings.ToLower(req.Permission))

	if req.FileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(
			w,
			http.StatusBadRequest,
			"อีเมลผู้ใช้ที่ต้องการแชร์ไม่ถูกต้อง",
		)
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(
		r.Context(),
		req.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบผู้ใช้จากอีเมลนี้",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบผู้ใช้ที่ต้องการแชร์ได้",
		)
		return
	}

	if sharedWithUser.ID == claims.UserID {
		response.Error(
			w,
			http.StatusBadRequest,
			"ไม่สามารถแชร์ไฟล์ให้ตัวเองได้",
		)
		return
	}

	if req.Permission != "viewer" && req.Permission != "editor" {
		response.Error(
			w,
			http.StatusBadRequest,
			"สิทธิ์ต้องเป็น viewer หรือ editor",
		)
		return
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		response.Error(
			w,
			http.StatusBadRequest,
			"วันหมดอายุต้องเป็นเวลาในอนาคต",
		)
		return
	}

	if _, err := h.fileRepo.FindByIDAndUserID(
		r.Context(),
		req.FileID,
		claims.UserID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ที่ต้องการแชร์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบไฟล์ได้",
		)
		return
	}

	sharedFile := &models.SharedFile{
		FileID:           req.FileID,
		SharedByUserID:   claims.UserID,
		SharedWithUserID: sharedWithUser.ID,
		Permission:       req.Permission,
		ExpiresAt:        req.ExpiresAt,
	}

	if err := h.sharedFileRepo.Create(
		r.Context(),
		sharedFile,
	); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถแชร์ไฟล์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		map[string]interface{}{
			"message":     "แชร์ไฟล์สำเร็จ",
			"shared_file": models.NewSharedFileResponse(sharedFile),
		},
	)
}

func (h *SharedFileHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	files, err := h.sharedFileRepo.FindSharedFilesByUserID(
		r.Context(),
		claims.UserID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดรายการไฟล์ที่แชร์กันได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดรายการไฟล์ที่แชร์สำเร็จ",
			"files":   files,
			"total":   len(files),
		},
	)
}

func (h *SharedFileHandler) ListUserPermissionInFile(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodGet {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(
		r.URL.Query().Get("file_id"),
		10,
		64,
	)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	userPermissions, err :=
		h.sharedFileRepo.FindAllUserPermissionInFile(
			r.Context(),
			fileID,
			claims.UserID,
		)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดรายการสิทธิ์การแชร์ไฟล์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":     "โหลดรายการสิทธิ์การแชร์ไฟล์สำเร็จ",
			"permissions": userPermissions,
			"total":       len(userPermissions),
		},
	)
}

func (h *SharedFileHandler) UpdatePermissionUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPatch {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	var req models.SharedFileUpdatePermissionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	req.Permission = strings.TrimSpace(strings.ToLower(req.Permission))

	if req.FileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(
			w,
			http.StatusBadRequest,
			"อีเมลผู้ใช้ที่ต้องการแชร์ไม่ถูกต้อง",
		)
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(
		r.Context(),
		req.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบผู้ใช้จากอีเมลนี้",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบผู้ใช้ที่ต้องการแชร์ได้",
		)
		return
	}

	if sharedWithUser.ID == claims.UserID {
		response.Error(
			w,
			http.StatusBadRequest,
			"ไม่สามารถแชร์ไฟล์ให้ตัวเองได้",
		)
		return
	}

	if req.Permission != "viewer" && req.Permission != "editor" {
		response.Error(
			w,
			http.StatusBadRequest,
			"สิทธิ์ต้องเป็น viewer หรือ editor",
		)
		return
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		response.Error(
			w,
			http.StatusBadRequest,
			"วันหมดอายุต้องเป็นเวลาในอนาคต",
		)
		return
	}

	fileID := req.FileID
	if fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	sharedFilePermission := &models.SharedFile{
		Permission:       req.Permission,
		ExpiresAt:        req.ExpiresAt,
		FileID:           fileID,
		SharedByUserID:   claims.UserID,
		SharedWithUserID: sharedWithUser.ID,
	}

	updatePermissioUser, err := h.sharedFileRepo.UpdatePermissionUserFile(
		r.Context(),
		sharedFilePermission,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถอัพเดตสิทธิ์การเข้าถึงได้",
		)

		return
	}

	if !updatePermissioUser {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบผู้ใช้งานที่สามารถแก้ไขสิทธิ์ได้",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "อัปเดตสิทธิ์การเข้าถึงไฟล์สำเร็จ",
			"shared_file": map[string]interface{}{
				"file_id":             fileID,
				"shared_by_user_id":   claims.UserID,
				"shared_with_user_id": sharedWithUser.ID,
				"permission":          req.Permission,
				"expires_at":          req.ExpiresAt,
			},
		},
	)

}

func (h *SharedFileHandler) RemoveSharedUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodDelete {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)

		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่่ระบบก่อนใช้งาน",
		)
		return
	}

	var req models.SharedFileUpdatePermissionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	sharedFileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || sharedFileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสแชร์ไฟล์ไม่ถูกต้อง",
		)

		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(
			w,
			http.StatusBadRequest,
			"อีเมลผู้ใช้ที่ต้องการแชร์ไม่ถูกต้อง",
		)
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(
		r.Context(),
		req.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบผู้ใช้จากอีเมลนี้",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบผู้ใช้ที่ต้องการแชร์ได้",
		)
		return
	}

	RemovedSharedUser, err := h.sharedFileRepo.RemoveSharedUser(
		r.Context(),
		sharedFileID,
		claims.UserID,
		sharedWithUser.ID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถลบสิทธิ์ผู้ใช้คนอื่นได้",
		)
		return
	}

	if !RemovedSharedUser {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบสิทธิ์ผู้ใช้ที่ต้องการลบ",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "อัปเดตสิทธิ์การเข้าถึงไฟล์สำเร็จ",
		},
	)

}

func (h *SharedFileHandler) SearchUsers(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(keyword)) < 2 {
		response.JSON(
			w,
			http.StatusOK,
			map[string]interface{}{
				"message": "โหลดรายชื่อผู้ใช้สำเร็จ",
				"users":   []models.UserResponse{},
				"total":   0,
			},
		)
		return
	}

	users, err := h.userRepo.SearchByEmailOrName(
		r.Context(),
		keyword,
		claims.UserID,
		8,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถค้นหาผู้ใช้ได้",
		)
		return
	}

	userResponses := make([]models.UserResponse, 0, len(users))
	for index := range users {
		userResponses = append(
			userResponses,
			models.NewUserResponse(&users[index]),
		)
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดรายชื่อผู้ใช้สำเร็จ",
			"users":   userResponses,
			"total":   len(userResponses),
		},
	)
}

// CreatePublicLink creates an "anyone with the link" share URL for a file.
// The API stores only a hashed token, while the raw token is returned in the URL.
func (h *SharedFileHandler) CreatePublicLink(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		64*1024,
	)

	var req models.CreatePublicFileLinkRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	if req.FileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		response.Error(
			w,
			http.StatusBadRequest,
			"วันหมดอายุต้องเป็นเวลาในอนาคต",
		)
		return
	}

	if _, err := h.fileRepo.FindByIDAndUserID(
		r.Context(),
		req.FileID,
		claims.UserID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ที่ต้องการแชร์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบไฟล์ได้",
		)
		return
	}

	rawToken, err := auth.GenerateVerificationToken()
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถสร้างลิงก์แชร์ได้",
		)
		return
	}

	publicLink := &models.PublicFileLink{
		FileID:     req.FileID,
		UserID:     claims.UserID,
		TokenHash:  auth.HashToken(rawToken),
		Permission: "viewer",
		ExpiresAt:  req.ExpiresAt,
	}

	if err := h.publicFileLinkRepo.Create(
		r.Context(),
		publicLink,
	); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถสร้างลิงก์แชร์ได้",
		)
		return
	}

	shareURL := h.buildPublicFileLinkURL(
		r,
		rawToken,
	)

	response.JSON(
		w,
		http.StatusCreated,
		map[string]interface{}{
			"message":     "สร้างลิงก์แชร์สำเร็จ",
			"public_link": models.NewPublicFileLinkResponse(publicLink, shareURL),
		},
	)
}

// DownloadPublicLink lets anyone who has a valid token download the shared file.
func (h *SharedFileHandler) DownloadPublicLink(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	rawToken := strings.TrimSpace(r.PathValue("token"))
	if rawToken == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"ลิงก์แชร์ไม่ถูกต้อง",
		)
		return
	}

	_, userFile, err := h.publicFileLinkRepo.FindActiveByTokenHash(
		r.Context(),
		auth.HashToken(rawToken),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ลิงก์แชร์หมดอายุหรือไม่พบไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดไฟล์จากลิงก์แชร์ได้",
		)
		return
	}

	h.serveSharedFile(
		w,
		r,
		userFile,
	)
}

func (h *SharedFileHandler) serveSharedFile(
	w http.ResponseWriter,
	r *http.Request,
	userFile *models.UserFile,
) {
	storagePath, err := h.resolveStoragePath(userFile.StoragePath)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่พบที่จัดเก็บไฟล์ในเซิร์ฟเวอร์",
		)
		return
	}

	storedFile, err := os.Open(storagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถเปิดไฟล์ได้",
		)
		return
	}
	defer storedFile.Close()

	fileInfo, err := storedFile.Stat()
	if err != nil || fileInfo.IsDir() {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบไฟล์",
		)
		return
	}

	contentType := userFile.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set(
		"Content-Disposition",
		mime.FormatMediaType(
			"attachment",
			map[string]string{
				"filename": userFile.OriginalName,
			},
		),
	)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")

	http.ServeContent(
		w,
		r,
		userFile.OriginalName,
		fileInfo.ModTime(),
		storedFile,
	)
}

func (h *SharedFileHandler) buildPublicFileLinkURL(
	r *http.Request,
	token string,
) string {
	scheme := "http"
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	} else if r.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf(
		"%s://%s/api/public/files/%s/download",
		scheme,
		r.Host,
		token,
	)
}

// resolveStoragePath keeps public downloads inside the configured upload root.
func (h *SharedFileHandler) resolveStoragePath(
	storagePath string,
) (string, error) {
	cleanStoragePath := filepath.Clean(
		filepath.FromSlash(storagePath),
	)

	uploadRoot := h.uploadRoot
	if uploadRoot == "" {
		uploadRoot = "uploads"
	}

	uploadRootAbsolute, err := filepath.Abs(uploadRoot)
	if err != nil {
		return "", err
	}

	fileAbsolute, err := filepath.Abs(cleanStoragePath)
	if err != nil {
		return "", err
	}

	relativePath, err := filepath.Rel(
		uploadRootAbsolute,
		fileAbsolute,
	)
	if err != nil {
		return "", err
	}

	if relativePath == ".." ||
		strings.HasPrefix(
			relativePath,
			".."+string(os.PathSeparator),
		) ||
		filepath.IsAbs(relativePath) {
		return "", errors.New("storage path is outside upload root")
	}

	return fileAbsolute, nil
}
