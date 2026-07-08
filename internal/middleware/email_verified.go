package middleware

import (
	"database/sql"
	"errors"
	"net/http"

	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

func RequireVerifiedEmail(
	userRepo *repository.UserRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			claims, ok := UserFromContext(r.Context())
			if !ok {
				response.Error(
					w,
					http.StatusUnauthorized,
					"authentication required",
				)
				return
			}

			user, err := userRepo.FindByID(
				r.Context(),
				claims.UserID,
			)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					response.Error(
						w,
						http.StatusUnauthorized,
						"user not found",
					)
					return
				}

				response.Error(
					w,
					http.StatusInternalServerError,
					"cannot verify user email status",
				)
				return
			}

			if user.EmailVerifiedAt == nil {
				response.Error(
					w,
					http.StatusForbidden,
					"please verify your email before using this resource",
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
