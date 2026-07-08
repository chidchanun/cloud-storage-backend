package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config ใช้เก็บค่าตั้งค่าทั้งหมดของระบบ
// เช่น port, database config, JWT config
type Config struct {
	AppPort string
	AppEnv  string

	CORSOrigin string

	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string

	JWTSecret    string
	JWTExpiresIn time.Duration

	UploadRoot      string
	MaxUploadSize   int64
	ChunkUploadSize int64

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	FrontendAuthSuccessURL string
	FrontendAuthErrorURL   string

	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string

	EmailVerificationURL          string
	FrontendEmailVerifySuccessURL string
	FrontendEmailVerifyErrorURL   string
}

// Load อ่านค่าจาก .env แล้วคืนค่าเป็น Config
func Load() Config {
	loadEnvFile()

	// แปลง JWT_EXPIRES_IN จาก string เช่น "1h" เป็น time.Duration
	jwtExpiresIn, err := time.ParseDuration(getEnv("JWT_EXPIRES_IN", "1h"))
	if err != nil {
		jwtExpiresIn = time.Hour
	}

	return Config{
		AppPort: getEnv("APP_PORT", "8080"),
		AppEnv:  getEnv("APP_ENV", "development"),

		CORSOrigin: getEnv(
			"CORS_ORIGIN",
			"http://localhost:4200",
		),

		MySQLHost:     getEnv("MYSQL_HOST", "127.0.0.1"),
		MySQLPort:     getEnv("MYSQL_PORT", "3306"),
		MySQLUser:     getEnv("MYSQL_USER", "root"),
		MySQLPassword: getEnv("MYSQL_PASSWORD", ""),
		MySQLDatabase: getEnv("MYSQL_DATABASE", ""),

		JWTSecret:    getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpiresIn: jwtExpiresIn,

		UploadRoot: getEnv(
			"UPLOAD_ROOT",
			"uploads",
		),

		MaxUploadSize: getEnvInt64(
			"MAX_UPLOAD_SIZE",
			25<<20,
		),

		ChunkUploadSize: getEnvInt64(
			"CHUNK_UPLOAD_SIZE",
			25<<20,
		),

		GoogleClientID: getEnv(
			"GOOGLE_CLIENT_ID",
			"",
		),

		GoogleClientSecret: getEnv(
			"GOOGLE_CLIENT_SECRET",
			"",
		),

		GoogleRedirectURL: getEnv(
			"GOOGLE_REDIRECT_URL",
			"http://localhost:8080/api/auth/google/callback",
		),

		FrontendAuthSuccessURL: getEnv(
			"FRONTEND_AUTH_SUCCESS_URL",
			"http://localhost:4200/auth/callback",
		),

		FrontendAuthErrorURL: getEnv(
			"FRONTEND_AUTH_ERROR_URL",
			"http://localhost:4200/login",
		),

		SMTPHost: getEnv(
			"SMTP_HOST",
			"",
		),

		SMTPPort: getEnv(
			"SMTP_PORT",
			"587",
		),

		SMTPUsername: getEnv(
			"SMTP_USERNAME",
			"",
		),

		SMTPPassword: getEnv(
			"SMTP_PASSWORD",
			"",
		),

		SMTPFromName: getEnv(
			"SMTP_FROM_NAME",
			"AnuCloud",
		),

		SMTPFromEmail: getEnv(
			"SMTP_FROM_EMAIL",
			"",
		),

		EmailVerificationURL: getEnv(
			"EMAIL_VERIFICATION_URL",
			"http://localhost:8080/api/auth/verify-email",
		),

		FrontendEmailVerifySuccessURL: getEnv(
			"FRONTEND_EMAIL_VERIFY_SUCCESS_URL",
			"http://localhost:4200/dashboard?verified=1",
		),

		FrontendEmailVerifyErrorURL: getEnv(
			"FRONTEND_EMAIL_VERIFY_ERROR_URL",
			"http://localhost:4200/login?error=email_verification_failed",
		),
	}
}

func loadEnvFile() {
	envFile := os.Getenv("ENV_FILE")
	if envFile != "" {
		_ = godotenv.Load(envFile)
		return
	}

	_ = godotenv.Load(".env.local", ".env")
}

// getEnv ใช้อ่านค่าจาก environment variable
// ถ้าไม่มีค่า จะใช้ fallback แทน
func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	parsedValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsedValue <= 0 {
		return fallback
	}

	return parsedValue
}
