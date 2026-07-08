# AnuCloud Backend Run Modes

## Local

1. Copy `.env.local.example` to `.env.local`.
2. Fill database, Google OAuth, and SMTP values.
3. Run with Go:

```powershell
$env:ENV_FILE=".env.local"
go run ./cmd/api
```

Or run with Docker Compose:

```powershell
docker compose -f compose.local.yml up --build
```

## Deploy

1. Copy `.env.deploy.example` to `.env.deploy` on the server.
2. Fill production domains and secrets.
3. Build the Docker image:

```powershell
docker build -t anucloud-backend:latest .
```

4. Run with Docker Compose:

```powershell
docker compose -f compose.deploy.yml up -d
```

## Notes

- `.env`, `.env.local`, and `.env.deploy` are ignored by Docker build context.
- Local example uses `APP_PORT=8080`.
- Deploy example uses `APP_PORT=4002` for `https://cloudstorageapi.chidchanun.online`.
- Production cookies become secure when `APP_ENV=production`.
