package auth

import (
	"errors"
	"net/http"
	"time"
)

const AccessTokenCookieName = "access_token"

// CookieService ใช้จัดการ JWT ที่เก็บอยู่ใน cookie
type CookieService struct {
	secure    bool
	expiresIn time.Duration
	sameSite  http.SameSite
}

// NewCookieService สร้าง service สำหรับจัดการ cookie
func NewCookieService(
	secure bool,
	expiresIn time.Duration,
	sameSite http.SameSite,
) *CookieService {
	return &CookieService{
		secure:    secure,
		expiresIn: expiresIn,
		sameSite:  sameSite,
	}
}

// SetAccessToken ใช้บันทึก access token ลง HttpOnly cookie
func (s *CookieService) SetAccessToken(
	w http.ResponseWriter,
	token string,
	expiredAt time.Time,
) {
	maxAge := int(time.Until(expiredAt).Seconds())

	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
		MaxAge:   maxAge,
		Expires:  expiredAt,
	})
}

// GetAccessToken อ่าน access token จาก request cookie
func (s *CookieService) GetAccessToken(
	r *http.Request,
) (string, error) {
	cookie, err := r.Cookie(AccessTokenCookieName)

	if err != nil {
		return "", err
	}

	if cookie.Value == "" {
		return "", errors.New("โปรดเข้าสู่ระบบก่อน")
	}

	return cookie.Value, nil
}

func (s *CookieService) ClearAccessToken(
	w http.ResponseWriter,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
