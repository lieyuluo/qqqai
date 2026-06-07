package middleware

import (
	"context"
	"net/http"
	"strings"

	"qqqai/config"
	"qqqai/internal/dao"
	"qqqai/internal/entity"
	"qqqai/internal/response"
	"qqqai/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
)

type contextKey string

const userContextKey contextKey = "current_user"

func RequestID(r *ghttp.Request) {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	r.Request.Header.Set("X-Request-ID", requestID)
	r.Response.Header().Set("X-Request-ID", requestID)
	r.Middleware.Next()
}

func CORS(r *ghttp.Request) {
	origin := r.Header.Get("Origin")
	allowedOrigins := config.GetCORSAllowedOrigins()
	allowOrigin := ""
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			allowOrigin = "*"
			break
		}
		if allowed == origin {
			allowOrigin = origin
			break
		}
	}
	if allowOrigin == "" && origin != "" {
		response.Forbidden(r, "origin is not allowed")
		return
	}
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	r.Response.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	r.Response.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	r.Response.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
	r.Response.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	if r.Method == http.MethodOptions {
		r.Response.WriteStatus(http.StatusNoContent)
		return
	}
	r.Middleware.Next()
}

func Recovery(r *ghttp.Request) {
	defer func() {
		if recover() != nil {
			response.Internal(r, "internal server error")
		}
	}()
	r.Middleware.Next()
}

func Auth(r *ghttp.Request) {
	tokenText := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(tokenText), "bearer ") {
		tokenText = strings.TrimSpace(tokenText[7:])
	}
	claims, err := service.ParseToken(tokenText)
	if err != nil {
		response.Unauthorized(r, "invalid token")
		return
	}
	user, err := dao.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user.Status != entity.UserStatusActive {
		response.Unauthorized(r, "user is not available")
		return
	}
	ctx := context.WithValue(r.Context(), userContextKey, user)
	r.Request = r.Request.WithContext(ctx)
	r.Middleware.Next()
}

func Admin(r *ghttp.Request) {
	user := CurrentUser(r)
	if user == nil || user.Role != entity.RoleAdmin {
		response.Forbidden(r, "admin permission required")
		return
	}
	r.Middleware.Next()
}

func CurrentUser(r *ghttp.Request) *entity.User {
	value := r.Context().Value(userContextKey)
	if user, ok := value.(*entity.User); ok {
		return user
	}
	return nil
}
