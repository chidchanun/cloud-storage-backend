package auth

import (
	"strconv"
	"time"

	"cloud-storage-backend/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// Claims คือข้อมูลที่เราจะฝังไว้ใน JWT
type Claims struct {
	UserID    int    `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	// RegisteredClaims คือ claims มาตรฐาน เช่น exp, iat, iss, sub
	jwt.RegisteredClaims
}

// JWTService ใช้จัดการ generate และ parse JWT
type JWTService struct {
	secret    []byte
	expiresIn time.Duration
	issuer    string
}

// NewJWTService สร้าง JWT service
func NewJWTService(secret string, expiresIn time.Duration, issuer string) *JWTService {
	return &JWTService{
		secret:    []byte(secret),
		expiresIn: expiresIn,
		issuer:    issuer,
	}
}

// Generate ใช้สร้าง access token ให้ user
func (s *JWTService) Generate(
	user *models.User,
) (string, time.Time, error) {
	now := time.Now().UTC()
	expiredAt := now.Add(s.expiresIn)

	claims := Claims{
		UserID:    user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(user.ID),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiredAt),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiredAt, nil
}

// Parse ใช้ตรวจสอบ token และดึง claims ออกมา
func (s *JWTService) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			// คืน secret ที่ใช้ตรวจลายเซ็น token
			return s.secret, nil
		},

		// จำกัดให้รับเฉพาะ algorithm HS256
		// ป้องกันการสลับ algorithm จาก client
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
