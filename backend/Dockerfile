# ---- Stage 1: build ----
FROM golang:1.25-alpine AS builder

# Зависимости для CGO (pgx не требует CGO, но migrate может)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Кэшируем зависимости отдельным слоем
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Статический бинарник — без внешних .so зависимостей
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/server ./cmd/server

# ---- Stage 2: runtime ----
FROM scratch

# Корневые сертификаты для HTTPS-запросов (MinIO, внешние API)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Часовые пояса
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /app/server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
