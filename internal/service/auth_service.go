package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"qqqai/config"
	"qqqai/internal/dao"
	"qqqai/internal/entity"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RegisterInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("valid email is required")
	}
	if username == "" {
		username = strings.Split(email, "@")[0]
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return dao.CreateUser(ctx, email, username, hash, entity.RoleUser)
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*entity.User, string, error) {
	user, err := dao.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("invalid email or password")
		}
		return nil, "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, "", fmt.Errorf("invalid email or password")
	}
	token, err := IssueToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func EnsureAdmin(ctx context.Context) error {
	if config.GlobalConfig == nil {
		return nil
	}
	email := strings.TrimSpace(config.GlobalConfig.Web.AdminEmail)
	password := strings.TrimSpace(config.GlobalConfig.Web.AdminPassword)
	if email == "" || password == "" {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return dao.UpsertAdmin(ctx, email, "Admin", hash)
}

func HashPassword(password string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(data), err
}

func IssueToken(user *entity.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(config.GetJWTExpireHours()) * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.GetJWTSecret()))
}

func ParseToken(tokenText string) (*Claims, error) {
	tokenText = strings.TrimSpace(tokenText)
	if tokenText == "" {
		return nil, fmt.Errorf("missing token")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(config.GetJWTSecret()), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
