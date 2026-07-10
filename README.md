# AnuCloud Backend

Go API server for AnuCloud, a cloud storage application with files, folders, sharing, trash, starred items, profile management, Google OAuth, email verification, and plan limits.

## Stack

- Go 1.25
- MySQL
- Native `net/http` router
- JWT authentication
- Cookie-based sessions
- Google OAuth
- SMTP email verification
- Docker

## Main Features

- Email/password authentication
- Google OAuth login
- Email verification
- Multi-device login
- Profile update and profile picture upload
- Set password after Google login
- File upload and chunk upload
- File download
- Folders
- Rename, move, soft delete, restore, and permanent delete
- Shared files and shared folders
- Public file links
- Starred files and starred folders
- Recent files
- Trash
- User plans and storage usage limits

## Requirements

- Go 1.25+
- MySQL 8+
- Docker, optional
- SMTP account, optional but required for email verification
- Google OAuth credentials, optional but required for Google login

## Environment Files

The app reads configuration from environment variables. The current project uses:

```text
.env.local        Local development
.env.production   Production deployment
.env              Current/default local file
```

Never commit real secrets to Git.

Important variables:

```env
APP_PORT=4003
APP_ENV=production
CORS_ORIGIN=https://cloudstorage.chidchanun.online

MYSQL_HOST=host.docker.internal
MYSQL_PORT=3306
MYSQL_DATABASE=cloudstorage_db
MYSQL_USER=your_mysql_user
MYSQL_PASSWORD=your_mysql_password

JWT_SECRET=replace_with_a_long_random_secret
JWT_EXPIRES_IN=24h

UPLOAD_ROOT=/app/uploads
MAX_UPLOAD_SIZE=10000000000
CHUNK_UPLOAD_SIZE=106432000

GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_REDIRECT_URL=https://cloudstorageapi.chidchanun.online/api/auth/google/callback

FRONTEND_AUTH_SUCCESS_URL=https://cloudstorage.chidchanun.online/auth/callback
FRONTEND_AUTH_ERROR_URL=https://cloudstorage.chidchanun.online/login

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_smtp_username
SMTP_PASSWORD=your_smtp_password
SMTP_FROM_NAME=AnuCloud
SMTP_FROM_EMAIL=no-reply@example.com

EMAIL_VERIFICATION_URL=https://cloudstorageapi.chidchanun.online/api/auth/verify-email
FRONTEND_EMAIL_VERIFY_SUCCESS_URL=https://cloudstorage.chidchanun.online/dashboard?verified=1
FRONTEND_EMAIL_VERIFY_ERROR_URL=https://cloudstorage.chidchanun.online/login?error=email_verification_failed
```

If an env value contains spaces, wrap it in quotes:

```env
SMTP_PASSWORD="my app password"
```

## Database

The full schema is available at:

```text
db/schema.sql
```

Additional migrations are in:

```text
db/migrations
```

Current migration files include:

```text
001_email_verification.sql
002_public_file_link.sql
003_shared_folder.sql
004_plan_user_plan.sql
005_allow_multiple_user_tokens.sql
005_user_folder_star.sql
```

Apply the schema/migrations to the MySQL database before running the API.

Example:

```bash
mysql -u root -p cloudstorage_db < db/schema.sql
```

Then apply any migration files that are not already included in your database.

## Run Locally

PowerShell:

```powershell
$env:ENV_FILE=".env.local"
go run ./cmd/api
```

Bash:

```bash
ENV_FILE=.env.local go run ./cmd/api
```

Default local API URL depends on `APP_PORT`.

Example:

```text
http://localhost:8080/api
```

## Test

```bash
go test ./...
```

## Build Binary

```bash
go build -trimpath -ldflags="-s -w" -o cloud-storage-api ./cmd/api
```

Windows output:

```powershell
go build -trimpath -ldflags="-s -w" -o cloud-storage-api.exe ./cmd/api
```

## Docker

Build image:

```bash
docker build -t anucloud-backend:latest .
```

Run container:

```bash
docker run -d \
  --name anucloud-backend \
  --restart unless-stopped \
  --env-file .env.production \
  -p 4003:4003 \
  -v anucloud_uploads:/app/uploads \
  anucloud-backend:latest
```

PowerShell:

```powershell
docker run -d `
  --name anucloud-backend `
  --restart unless-stopped `
  --env-file .env.production `
  -p 4003:4003 `
  -v anucloud_uploads:/app/uploads `
  anucloud-backend:latest
```

Check logs:

```bash
docker logs -f anucloud-backend
```

## Cloudflare Tunnel

Recommended public hostname mapping:

```text
cloudstorageapi.chidchanun.online -> http://anucloud-backend:4003
```

If `cloudflared` is not in the same Docker network:

```text
cloudstorageapi.chidchanun.online -> http://host.docker.internal:4003
```

Frontend production config should point to:

```text
https://cloudstorageapi.chidchanun.online/api
```

## Upload Notes

- Normal file upload uses `POST /api/files`.
- Large upload uses chunk upload:
  - `POST /api/files/chunk-upload/start`
  - `POST /api/files/chunk-upload/{upload_id}/chunks/{index}`
  - `POST /api/files/chunk-upload/{upload_id}/complete`
- `CHUNK_UPLOAD_SIZE` controls chunk size in bytes.
- When using Cloudflare Tunnel, one in-flight chunk is usually more stable than parallel chunk requests.

## API Overview

Authentication:

```text
POST /api/auth/register
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/google
GET  /api/auth/google/callback
GET  /api/auth/verify-email
GET  /api/me
PATCH /api/me/password
PATCH /api/profile
```

Plans:

```text
GET  /api/plans
GET  /api/me/plan
POST /api/me/plan
```

Files:

```text
POST   /api/files
GET    /api/files
GET    /api/files/search
GET    /api/files/{id}
DELETE /api/files/{id}
DELETE /api/files/{id}/delete
GET    /api/files/{id}/download
PATCH  /api/files/{id}/rename
PATCH  /api/files/{id}/move
```

File sharing:

```text
POST   /api/files/share-file
GET    /api/files/share-file
GET    /api/files/share-file/permissions
PATCH  /api/files/share-file/permissions
DELETE /api/files/share-file/permissions/{id}
POST   /api/files/share-link
GET    /api/public/files/{token}/download
```

Trash:

```text
GET   /api/trash/files
GET   /api/trash/files/{id}
PATCH /api/trash/files/{id}/restore
```

Starred files:

```text
GET    /api/files/starred
POST   /api/files/{id}/star
DELETE /api/files/{id}/star
GET    /api/files/{id}/star
```

Folders:

```text
POST  /api/folders
GET   /api/folders
GET   /api/folders/{id}
PATCH /api/folders/{id}/rename
PATCH /api/folders/{id}/move
PATCH /api/folders/{id}/delete
```

Folder sharing:

```text
POST   /api/folders/share-folder
GET    /api/folders/share-folder
GET    /api/folders/share-folder/permissions
PATCH  /api/folders/share-folder/permissions
DELETE /api/folders/share-folder/permissions/{id}
```

Starred folders:

```text
GET    /api/folders/starred
POST   /api/folders/{id}/star
DELETE /api/folders/{id}/star
GET    /api/folders/{id}/star
```

Users:

```text
GET /api/users/search
```

## Production Checklist

- Use a strong `JWT_SECRET`.
- Use `APP_ENV=production`.
- Set `CORS_ORIGIN` to the exact frontend origin.
- Make sure frontend and backend domains use HTTPS.
- Use secure cookies for cross-subdomain auth.
- Persist `/app/uploads` with a Docker volume.
- Backup MySQL and upload storage.
- Keep `.env.production` out of Git.
- Confirm Cloudflare Tunnel points to the same port as `APP_PORT`.
