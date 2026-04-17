package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/borrowtime/server/internal/usecase"
)

// contextKey — тип для ключей контекста
type contextKey string

const (
	ctxUserID   contextKey = "userID"
	ctxUserRole contextKey = "userRole"
)

// respondJSON сериализует v в JSON и отправляет ответ
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// respondError отправляет JSON с полем "error"
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// userIDFromCtx извлекает ID пользователя из контекста (устанавливается JWT middleware)
func userIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

// userRoleFromCtx извлекает роль пользователя из контекста
func userRoleFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserRole).(string)
	return v
}

// realIP получает IP-адрес клиента с учётом прокси-заголовков
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// JWTMiddleware проверяет Bearer-токен и кладёт claims в контекст запроса.
// Если токен отсутствует или невалиден — запрос продолжается без аутентификации
// (защищённые маршруты сами проверяют userIDFromCtx).
func JWTMiddleware(authUC *usecase.AuthUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := authUC.ValidateAccessJWT(tokenStr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORSMiddleware обрабатывает CORS-заголовки для фронтенда
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
