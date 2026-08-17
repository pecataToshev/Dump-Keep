FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o dump-keep .

FROM alpine:3.23

LABEL org.opencontainers.image.title="Dump-Keep" \
    org.opencontainers.image.description="Encrypted PostgreSQL backups shipped to Google Drive or any S3-compatible storage." \
    org.opencontainers.image.source="https://github.com/pecataToshev/Dump-Keep" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.authors="pecata.toshev@gmail.com"

# postgresql18-client provides pg_dump / pg_dumpall — newer clients can
# dump older servers, so this covers PostgreSQL 15, 16, 17 and 18.
RUN apk add --no-cache ca-certificates postgresql18-client
WORKDIR /app
COPY --from=builder /app/dump-keep /app/dump-keep
CMD ["/app/dump-keep"]
