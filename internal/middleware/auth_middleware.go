package middleware

import (
	"context"
	"net/http"
	"strings"

	"cloud-storage-backend/internal/auth"
	"cloud-storage-backend/internal/response"
	"cloud-storage-backend/internal/repository"
)

type contextKey string

const userContextKey contextKey = "auth_user"

func Auth(
	jwtService *auth.JWTService,
	cookieService *auth.CookieService,
	userTokenRepo *repository.UserTokenRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			// พยายามอ่าน JWT จาก HttpOnly cookie ก่อน
			tokenString, err := cookieService.GetAccessToken(r)

			// ถ้าไม่มี cookie ให้รองรับ Authorization header ด้วย
			// ทำให้ยังสามารถทดสอบด้วย Postman หรือ mobile app ได้
			if err != nil {
				authHeader := r.Header.Get("Authorization")

				parts := strings.SplitN(authHeader, " ", 2)

				if len(parts) != 2 ||
					!strings.EqualFold(parts[0], "Bearer") {
					response.Error(
						w,
						http.StatusUnauthorized,
						"authentication required",
					)
					return
				}

				tokenString = parts[1]
			}

			// ตรวจสอบลายเซ็นและวันหมดอายุของ JWT
			claims, err := jwtService.Parse(tokenString)
			if err != nil {
				response.Error(
					w,
					http.StatusUnauthorized,
					"invalid or expired token",
				)
				return
			}

			tokenHash := auth.HashToken(tokenString)

			isValid, err := userTokenRepo.IsValid(
				r.Context(),
				tokenHash,
			)

			if err != nil {
				response.Error(
					w,
					http.StatusInternalServerError,
					"cannot validate session",
				)
				return
			}

			if !isValid {
				response.Error(
					w,
					http.StatusUnauthorized,
					"session expired or revoked",
				)
				return
			}

			// เก็บ claims ลง request context
			ctx := context.WithValue(
				r.Context(),
				userContextKey,
				claims,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(
	ctx context.Context,
) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userContextKey).(*auth.Claims)

	return claims, ok
}
