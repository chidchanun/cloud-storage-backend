# Compile the Go API as a small static Linux binary.
FROM golang:1.25.11-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cloud-storage-api ./cmd/api

# Runtime image keeps uploads writable and avoids running as root.
FROM alpine:3.22 AS runtime

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S app -G app

ENV APP_PORT=4003
ENV UPLOAD_ROOT=/app/uploads

RUN mkdir -p /app/uploads && chown -R app:app /app

COPY --from=build /out/cloud-storage-api /app/cloud-storage-api

USER app

EXPOSE 4002

CMD ["/app/cloud-storage-api"]
