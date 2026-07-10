package router

import (
	"net/http"

	"cloud-storage-backend/internal/auth"
	"cloud-storage-backend/internal/handler"
	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

type Config struct {
	AuthHandler           *handler.AuthHandler
	FileHandler           *handler.FileHandler
	FolderHandler         *handler.FolderHandler
	SharedFileHandler     *handler.SharedFileHandler
	SharedFolderHandler   *handler.SharedFolderHandler
	UserHandler           *handler.UserHandler
	PlanHandler           *handler.PlanHandler
	UserFileStarHandler   *handler.UserFileStarHandler
	UserFolderStarHandler *handler.UserFolderStarHandler

	JWTService          *auth.JWTService
	CookieService       *auth.CookieService
	UserRepository      *repository.UserRepository
	UserTokenRepository *repository.UserTokenRepository

	CORSOrigin string
	UploadRoot string
}

func New(cfg Config) http.Handler {
	// ตรวจสอบ dependency ตอนเปิด server
	if cfg.AuthHandler == nil {
		panic("router: AuthHandler is nil")
	}

	if cfg.FileHandler == nil {
		panic("router: FileHandler is nil")
	}

	if cfg.FolderHandler == nil {
		panic("router: FolderHandler is nil")
	}

	if cfg.SharedFileHandler == nil {
		panic("router: SharedFileHandler is nil")
	}

	if cfg.SharedFolderHandler == nil {
		panic("router: SharedFolderHandler is nil")
	}

	if cfg.UserHandler == nil {
		panic("router: UserHandler is nil")
	}

	if cfg.PlanHandler == nil {
		panic("router: PlanHandler is nil")
	}

	if cfg.JWTService == nil {
		panic("router: JWTService is nil")
	}

	if cfg.CookieService == nil {
		panic("router: CookieService is nil")
	}

	if cfg.UserTokenRepository == nil {
		panic("router: UserTokenRepository is nil")
	}

	if cfg.UserRepository == nil {
		panic("router: UserRepository is nil")
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc(
		"/api/health",
		func(w http.ResponseWriter, r *http.Request) {
			response.JSON(
				w,
				http.StatusOK,
				map[string]string{
					"message": "API is running",
				},
			)
		},
	)

	// Public authentication routes
	mux.HandleFunc(
		"/api/auth/register",
		cfg.AuthHandler.Register,
	)

	mux.HandleFunc(
		"/api/auth/login",
		cfg.AuthHandler.Login,
	)

	mux.HandleFunc(
		"/api/auth/logout",
		cfg.AuthHandler.Logout,
	)

	// =========================
	// Google Authentication
	// =========================

	// เริ่ม Google Login
	mux.HandleFunc(
		"GET /api/auth/google",
		cfg.AuthHandler.GoogleLogin,
	)

	// Google Redirect Callback
	mux.HandleFunc(
		"GET /api/auth/google/callback",
		cfg.AuthHandler.GoogleCallback,
	)

	mux.HandleFunc(
		"GET /api/auth/verify-email",
		cfg.AuthHandler.VerifyEmail,
	)

	protectedRoute := middleware.Auth(
		cfg.JWTService,
		cfg.CookieService,
		cfg.UserTokenRepository,
	)

	protectedVerifiedRoute := func(next http.Handler) http.Handler {
		return protectedRoute(
			middleware.RequireVerifiedEmail(cfg.UserRepository)(next),
		)
	}

	// Protected user route
	mux.Handle(
		"/api/me",
		protectedRoute(
			http.HandlerFunc(cfg.AuthHandler.Me),
		),
	)

	mux.Handle(
		"PATCH /api/me/password",
		protectedRoute(
			http.HandlerFunc(cfg.AuthHandler.SetPassword),
		),
	)

	mux.Handle(
		"GET /api/plans",
		http.HandlerFunc(cfg.PlanHandler.List),
	)

	mux.Handle(
		"GET /api/me/plan",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.PlanHandler.Current),
		),
	)

	mux.Handle(
		"POST /api/me/plan",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.PlanHandler.Select),
		),
	)

	// =========================
	// Update Profile
	// =========================

	mux.Handle(
		"PATCH /api/profile",
		protectedRoute(
			http.HandlerFunc(cfg.UserHandler.UpdateUserProfile),
		),
	)

	mux.Handle(
		"/api/me/profile-picture",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.AuthHandler.UploadProfilePicture),
		),
	)

	// Protected file upload route
	mux.Handle(
		"POST /api/files",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.Upload),
		),
	)

	mux.Handle(
		"POST /api/files/chunk-upload/start",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.StartChunkUpload),
		),
	)

	mux.Handle(
		"POST /api/files/chunk-upload/{upload_id}/chunks/{index}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.UploadChunk),
		),
	)

	mux.Handle(
		"POST /api/files/chunk-upload/{upload_id}/complete",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.CompleteChunkUpload),
		),
	)

	mux.Handle(
		"GET /api/files",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.List),
		),
	)

	mux.Handle(
		"GET /api/files/search",
		protectedRoute(
			http.HandlerFunc(
				cfg.FileHandler.SearchByFileName,
			),
		),
	)

	mux.Handle(
		"GET /api/files/{id}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.GetByID),
		),
	)

	mux.Handle(
		"DELETE /api/files/{id}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.SoftDelete),
		),
	)

	mux.Handle(
		"DELETE /api/files/{id}/delete",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.Delete),
		),
	)

	mux.Handle(
		"GET /api/files/{id}/download",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.Download),
		),
	)

	mux.Handle(
		"PATCH /api/files/{id}/rename",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.Rename),
		),
	)

	mux.Handle(
		"PATCH /api/files/{id}/move",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.MoveFile),
		),
	)

	mux.Handle(
		"POST /api/files/share-file",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.Create),
		),
	)

	mux.Handle(
		"GET /api/files/share-file",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.List),
		),
	)

	mux.Handle(
		"GET /api/files/share-file/permissions",
		protectedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.ListUserPermissionInFile),
		),
	)

	mux.Handle(
		"PATCH /api/files/share-file/permissions",
		protectedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.UpdatePermissionUser),
		),
	)

	mux.Handle(
		"DELETE /api/files/share-file/permissions/{id}",
		protectedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.RemoveSharedUser),
		),
	)

	mux.Handle(
		"GET /api/users/search",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.SearchUsers),
		),
	)

	mux.Handle(
		"POST /api/files/share-link",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFileHandler.CreatePublicLink),
		),
	)

	mux.Handle(
		"GET /api/public/files/{token}/download",
		http.HandlerFunc(cfg.SharedFileHandler.DownloadPublicLink),
	)

	// รายการไฟล์ในถังขยะ
	mux.Handle(
		"GET /api/trash/files",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.ListTrash),
		),
	)

	// ดูรายละเอียดไฟล์ในถังขยะ
	mux.Handle(
		"GET /api/trash/files/{id}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.GetTrashByID),
		),
	)

	// กู้คืนไฟล์จากถังขยะ
	mux.Handle(
		"PATCH /api/trash/files/{id}/restore",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FileHandler.Restore),
		),
	)

	// =========================
	// File Star API
	// =========================

	// รายการไฟล์สำคัญทั้งหมด
	// ต้องวางก่อน /api/files/{id}
	mux.Handle(
		"GET /api/files/starred",
		protectedRoute(
			http.HandlerFunc(
				cfg.UserFileStarHandler.ListStarredFiles,
			),
		),
	)

	// กดไฟล์เป็นสำคัญ
	mux.Handle(
		"POST /api/files/{id}/star",
		protectedRoute(
			http.HandlerFunc(
				cfg.UserFileStarHandler.StarFile,
			),
		),
	)

	// ยกเลิกไฟล์สำคัญ
	mux.Handle(
		"DELETE /api/files/{id}/star",
		protectedRoute(
			http.HandlerFunc(
				cfg.UserFileStarHandler.UnstarFile,
			),
		),
	)

	// ตรวจว่าไฟล์ถูกกดสำคัญหรือยัง
	mux.Handle(
		"GET /api/files/{id}/star",
		protectedRoute(
			http.HandlerFunc(
				cfg.UserFileStarHandler.CheckFileStar,
			),
		),
	)

	// =========================
	// Folder API
	// =========================

	// สร้างโฟลเดอร์
	mux.Handle(
		"POST /api/folders",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.Create),
		),
	)

	// ดูรายการโฟลเดอร์ใน Root หรือใน parent ที่ระบุ
	mux.Handle(
		"GET /api/folders",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.List),
		),
	)

	mux.Handle(
		"POST /api/folders/share-folder",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFolderHandler.Create),
		),
	)

	mux.Handle(
		"GET /api/folders/share-folder",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFolderHandler.List),
		),
	)

	mux.Handle(
		"GET /api/folders/share-folder/permissions",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFolderHandler.ListUserPermissionInFolder),
		),
	)

	mux.Handle(
		"PATCH /api/folders/share-folder/permissions",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFolderHandler.UpdatePermissionUser),
		),
	)

	mux.Handle(
		"DELETE /api/folders/share-folder/permissions/{id}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.SharedFolderHandler.RemoveSharedUser),
		),
	)

	// ดูรายละเอียดโฟลเดอร์
	mux.Handle(
		"GET /api/folders/starred",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.UserFolderStarHandler.ListStarredFolders),
		),
	)

	mux.Handle(
		"POST /api/folders/{id}/star",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.UserFolderStarHandler.StarFolder),
		),
	)

	mux.Handle(
		"DELETE /api/folders/{id}/star",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.UserFolderStarHandler.UnstarFolder),
		),
	)

	mux.Handle(
		"GET /api/folders/{id}/star",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.UserFolderStarHandler.CheckFolderStar),
		),
	)

	mux.Handle(
		"GET /api/folders/{id}",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.GetByID),
		),
	)

	// เปลี่ยนชื่อโฟลเดอร์
	mux.Handle(
		"PATCH /api/folders/{id}/rename",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.Rename),
		),
	)

	// ย้ายตำแหน่งโฟลเดอร์
	mux.Handle(
		"PATCH /api/folders/{id}/move",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.MoveFile),
		),
	)

	mux.Handle(
		"PATCH /api/folders/{id}/delete",
		protectedVerifiedRoute(
			http.HandlerFunc(cfg.FolderHandler.SoftDelete),
		),
	)

	uploadRoot := cfg.UploadRoot
	if uploadRoot == "" {
		uploadRoot = "uploads"
	}

	mux.Handle(
		"/uploads/profiles/",
		http.StripPrefix(
			"/uploads/",
			http.FileServer(http.Dir(uploadRoot)),
		),
	)

	return middleware.CORSMiddleware(mux, cfg.CORSOrigin)
}
