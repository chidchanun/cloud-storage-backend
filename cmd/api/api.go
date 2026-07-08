package main

import (
	"log"
	"net/http"
	"time"

	"cloud-storage-backend/internal/auth"
	"cloud-storage-backend/internal/config"
	"cloud-storage-backend/internal/database"
	"cloud-storage-backend/internal/email"
	"cloud-storage-backend/internal/handler"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/router"
)

func main() {
	cfg := config.Load()

	db, err := database.NewMySQL(cfg)
	if err != nil {
		log.Fatal("database connection error:", err)
	}
	defer db.Close()

	// Repository สำหรับตาราง user
	userRepo := repository.NewUserRepository(db)

	// ต้องประกาศตัวแปรนี้ก่อนนำไปใช้
	// Repository สำหรับตาราง user_token
	userTokenRepo := repository.NewUserTokenRepository(db)
	emailVerificationRepo := repository.NewEmailVerificationRepository(db)

	// Repository สำหรับจัดการ metadata ของไฟล์ใน MySQL
	fileRepo := repository.NewFileRepository(db)
	folderRepo := repository.NewFolderRepository(db)
	sharedFileRepo := repository.NewSharedFileRepository(db)
	sharedFolderRepo := repository.NewSharedFolderRepository(db)
	publicFileLinkRepo := repository.NewPublicFileLinkRepository(db)
	googleAuthRepo := repository.NewGoogleAuthRepository(db)
	planRepo := repository.NewPlanRepository(db)
	userPlanRepo := repository.NewUserPlanRepository(db)

	jwtService := auth.NewJWTService(
		cfg.JWTSecret,
		cfg.JWTExpiresIn,
		"cloud-storage-backend",
	)

	cookieSecure := cfg.AppEnv == "production"

	cookieService := auth.NewCookieService(
		cookieSecure,
		cfg.JWTExpiresIn,
		http.SameSiteLaxMode,
	)

	googleOAuthService := auth.NewGoogleOAuthService(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
		cookieSecure,
	)

	emailSender := email.NewSMTPSender(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUsername,
		cfg.SMTPPassword,
		cfg.SMTPFromName,
		cfg.SMTPFromEmail,
	)

	authHandler := handler.NewAuthHandler(
		userRepo,
		userTokenRepo,
		emailVerificationRepo,
		jwtService,
		cookieService,
		googleAuthRepo,
		googleOAuthService,
		emailSender,
		cfg.UploadRoot,
		cfg.MaxUploadSize,
		cfg.FrontendAuthSuccessURL,
		cfg.FrontendAuthErrorURL,
		cfg.EmailVerificationURL,
		cfg.FrontendEmailVerifySuccessURL,
		cfg.FrontendEmailVerifyErrorURL,
	)

	fileHandler := handler.NewFileHandler(
		fileRepo,
		folderRepo,
		sharedFileRepo,
		sharedFolderRepo,
		userPlanRepo,
		cfg.UploadRoot,
		cfg.MaxUploadSize,
		cfg.ChunkUploadSize,
	)

	folderHandler := handler.NewFolderHandler(
		folderRepo,
		fileRepo,
		sharedFolderRepo,
	)

	sharedFileHandler := handler.NewSharedFileHandler(
		sharedFileRepo,
		publicFileLinkRepo,
		fileRepo,
		userRepo,
		cfg.UploadRoot,
	)

	sharedFolderHandler := handler.NewSharedFolderHandler(
		sharedFolderRepo,
		folderRepo,
		userRepo,
	)

	userHandler := handler.NewUserHandler(
		userRepo,
		cfg.MaxUploadSize,
	)

	planHandler := handler.NewPlanHandler(
		planRepo,
		userPlanRepo,
	)

	appRouter := router.New(router.Config{
		AuthHandler:         authHandler,
		FileHandler:         fileHandler,
		FolderHandler:       folderHandler,
		SharedFileHandler:   sharedFileHandler,
		SharedFolderHandler: sharedFolderHandler,
		UserHandler:         userHandler,
		PlanHandler:         planHandler,
		JWTService:          jwtService,
		CookieService:       cookieService,
		UserRepository:      userRepo,
		UserTokenRepository: userTokenRepo,
		CORSOrigin:          cfg.CORSOrigin,
		UploadRoot:          cfg.UploadRoot,
	})

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           appRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Go API running on http://localhost:" + cfg.AppPort)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
