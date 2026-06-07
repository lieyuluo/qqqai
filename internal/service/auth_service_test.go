package service

import (
	"testing"

	"qqqai/config"
	"qqqai/internal/entity"
)

func TestIssueAndParseToken(t *testing.T) {
	config.GlobalConfig = &config.Config{
		Web: config.WebConfig{
			JWTSecret:      "test-secret",
			JWTExpireHours: 1,
		},
	}
	user := &entity.User{ID: 42, Email: "admin@example.com", Role: entity.RoleAdmin}
	token, err := IssueToken(user)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse token failed: %v", err)
	}
	if claims.UserID != user.ID || claims.Role != entity.RoleAdmin {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseTokenRejectsBadToken(t *testing.T) {
	config.GlobalConfig = &config.Config{
		Web: config.WebConfig{
			JWTSecret:      "test-secret",
			JWTExpireHours: 1,
		},
	}
	if _, err := ParseToken("not-a-token"); err == nil {
		t.Fatal("expected bad token to be rejected")
	}
}
