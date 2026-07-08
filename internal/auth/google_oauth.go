package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud-storage-backend/internal/models"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

const (
	// ชื่อ Cookie ที่ใช้เก็บ OAuth state ชั่วคราว
	googleOAuthStateCookieName = "google_oauth_state"

	// อายุของ state ไม่ควรนาน เพราะใช้แค่ระหว่างเริ่ม Login กับ Callback
	googleOAuthStateDuration = 10 * time.Minute
)

// GoogleOAuthService จัดการ Google OAuth และ OpenID Connect
type GoogleOAuthService struct {
	oauthConfig *oauth2.Config
	clientID    string

	// development ใช้ false
	// production ผ่าน HTTPS ใช้ true
	cookieSecure bool
}

// NewGoogleOAuthService สร้าง Service สำหรับ Google OAuth
func NewGoogleOAuthService(
	clientID string,
	clientSecret string,
	redirectURL string,
	cookieSecure bool,
) *GoogleOAuthService {
	return &GoogleOAuthService{
		clientID: clientID,
		cookieSecure: cookieSecure,

		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,

			// Endpoint มาตรฐานของ Google
			Endpoint: google.Endpoint,

			// openid ทำให้ได้รับ ID Token
			// email และ profile ใช้ขอข้อมูลผู้ใช้พื้นฐาน
			Scopes: []string{
				"openid",
				"email",
				"profile",
			},
		},
	}
}

// IsConfigured ตรวจว่า Google OAuth มีค่าที่จำเป็นครบหรือไม่
func (s *GoogleOAuthService) IsConfigured() bool {
	if s == nil || s.oauthConfig == nil {
		return false
	}

	return s.oauthConfig.ClientID != "" &&
		s.oauthConfig.ClientSecret != "" &&
		s.oauthConfig.RedirectURL != ""
}

// GenerateState สร้างค่า state แบบสุ่ม
// ใช้ป้องกัน OAuth CSRF
func (s *GoogleOAuthService) GenerateState() (string, error) {
	randomBytes := make([]byte, 32)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf(
			"generate Google OAuth state: %w",
			err,
		)
	}

	return base64.RawURLEncoding.EncodeToString(
		randomBytes,
	), nil
}

// SetStateCookie บันทึก state ลง HttpOnly Cookie ชั่วคราว
func (s *GoogleOAuthService) SetStateCookie(
	w http.ResponseWriter,
	state string,
) {
	expiresAt := time.Now().Add(googleOAuthStateDuration)

	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookieName,
		Value:    state,
		Path:     "/api/auth/google",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(googleOAuthStateDuration.Seconds()),
		Expires:  expiresAt,
	})
}

// ClearStateCookie ลบ state cookie หลังใช้งานเสร็จ
func (s *GoogleOAuthService) ClearStateCookie(
	w http.ResponseWriter,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookieName,
		Value:    "",
		Path:     "/api/auth/google",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

// ValidateState ตรวจว่า state จาก Google Callback
// ตรงกับ state ใน Cookie หรือไม่
func (s *GoogleOAuthService) ValidateState(
	r *http.Request,
	callbackState string,
) error {
	if callbackState == "" {
		return errors.New("Google OAuth state is missing")
	}

	stateCookie, err := r.Cookie(
		googleOAuthStateCookieName,
	)
	if err != nil {
		return errors.New("Google OAuth state cookie is missing")
	}

	if stateCookie.Value == "" {
		return errors.New("Google OAuth state cookie is empty")
	}

	// ใช้ ConstantTimeCompare เพื่อลดความเสี่ยงจาก timing attack
	if subtle.ConstantTimeCompare(
		[]byte(stateCookie.Value),
		[]byte(callbackState),
	) != 1 {
		return errors.New("Google OAuth state does not match")
	}

	return nil
}

// AuthorizationURL สร้าง URL สำหรับ Redirect ไปหน้า Google Login
func (s *GoogleOAuthService) AuthorizationURL(
	state string,
) string {
	return s.oauthConfig.AuthCodeURL(
		state,

		// ให้ผู้ใช้สามารถเลือก Google Account
		oauth2.SetAuthURLParam(
			"prompt",
			"select_account",
		),
	)
}

// ExchangeCode แลก authorization code เป็น OAuth Token
// แล้วตรวจสอบ Google ID Token
func (s *GoogleOAuthService) ExchangeCode(
	ctx context.Context,
	code string,
) (*models.GoogleIdentity, error) {
	if code == "" {
		return nil, errors.New(
			"Google authorization code is missing",
		)
	}

	// แลก authorization code เป็น OAuth token
	token, err := s.oauthConfig.Exchange(
		ctx,
		code,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"exchange Google authorization code: %w",
			err,
		)
	}

	// Google ส่ง ID Token มาใน extra field ชื่อ id_token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New(
			"Google response does not contain an ID token",
		)
	}

	// ตรวจ signature, issuer, expiration และ audience
	// audience ต้องตรงกับ Google Client ID ของระบบเรา
	payload, err := idtoken.Validate(
		ctx,
		rawIDToken,
		s.clientID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"validate Google ID token: %w",
			err,
		)
	}

	identity, err := googleIdentityFromClaims(
		payload.Claims,
	)
	if err != nil {
		return nil, err
	}

	return identity, nil
}

// googleIdentityFromClaims แปลง Claims จาก Google
// เป็น Model ที่ระบบเราใช้งาน
func googleIdentityFromClaims(
	claims map[string]interface{},
) (*models.GoogleIdentity, error) {
	subject := claimString(claims, "sub")
	email := claimString(claims, "email")

	if subject == "" {
		return nil, errors.New(
			"Google ID token does not contain sub",
		)
	}

	if email == "" {
		return nil, errors.New(
			"Google ID token does not contain email",
		)
	}

	emailVerified := claimBool(
		claims,
		"email_verified",
	)

	if !emailVerified {
		return nil, errors.New(
			"Google email is not verified",
		)
	}

	return &models.GoogleIdentity{
		Subject:       subject,
		Email:         email,
		EmailVerified: emailVerified,
		FirstName:     claimString(claims, "given_name"),
		LastName:      claimString(claims, "family_name"),
		FullName:      claimString(claims, "name"),
		PictureURL:    claimString(claims, "picture"),
	}, nil
}

// claimString อ่าน Claim ประเภท string
func claimString(
	claims map[string]interface{},
	key string,
) string {
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}

	return value
}

// claimBool อ่าน Claim ประเภท bool
func claimBool(
	claims map[string]interface{},
	key string,
) bool {
	value, ok := claims[key].(bool)
	if !ok {
		return false
	}

	return value
}