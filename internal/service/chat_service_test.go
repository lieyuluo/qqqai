package service

import (
	"context"
	"errors"
	"testing"
)

func TestChatServiceUsesFinalGraph(t *testing.T) {
	svc := NewChatServiceWithBackends(
		func(ctx context.Context, sessionID, query string) (string, error) {
			return "final reply", nil
		},
		func(ctx context.Context, sessionID, query string) (string, error) {
			return "fallback reply", nil
		},
	)
	resp, err := svc.Chat(context.Background(), ChatRequest{SessionID: "s1", Query: "hello"})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp.Reply != "final reply" {
		t.Fatalf("got %q", resp.Reply)
	}
}

func TestChatServiceFallsBackWhenFinalGraphFails(t *testing.T) {
	svc := NewChatServiceWithBackends(
		func(ctx context.Context, sessionID, query string) (string, error) {
			return "", errors.New("graph failed")
		},
		func(ctx context.Context, sessionID, query string) (string, error) {
			return "fallback reply", nil
		},
	)
	resp, err := svc.Chat(context.Background(), ChatRequest{SessionID: "s1", Query: "hello"})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp.Reply != "fallback reply" {
		t.Fatalf("got %q", resp.Reply)
	}
}
