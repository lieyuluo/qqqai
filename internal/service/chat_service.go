package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"qqqai/ai"
	"qqqai/flow"

	"github.com/cloudwego/eino/schema"
)

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
}

type ChatResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	LatencyMS int64  `json:"latency_ms"`
}

type ChatChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

type ChatBackend func(ctx context.Context, sessionID, query string) (string, error)

type ChatService struct {
	finalGraph ChatBackend
	fallback   ChatBackend
}

func NewChatService() *ChatService {
	return &ChatService{
		finalGraph: GenerateWithFinalGraph,
		fallback:   ai.GenerateReply,
	}
}

func NewChatServiceWithBackends(finalGraph, fallback ChatBackend) *ChatService {
	return &ChatService{finalGraph: finalGraph, fallback: fallback}
}

func (s *ChatService) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return ChatResponse{}, fmt.Errorf("query is required")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = "web:default"
	}

	started := time.Now()
	reply, err := s.finalGraph(ctx, sessionID, query)
	if err != nil {
		reply, err = s.fallback(ctx, sessionID, query)
	}
	if err != nil {
		return ChatResponse{}, err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return ChatResponse{}, fmt.Errorf("empty reply")
	}
	return ChatResponse{
		SessionID: sessionID,
		Reply:     reply,
		LatencyMS: time.Since(started).Milliseconds(),
	}, nil
}

func (s *ChatService) Stream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	ch := make(chan ChatChunk)
	go func() {
		defer close(ch)
		resp, err := s.Chat(ctx, req)
		if err != nil {
			select {
			case ch <- ChatChunk{Error: err.Error(), Done: true}:
			case <-ctx.Done():
			}
			return
		}
		for _, part := range splitForPseudoStream(resp.Reply, 24) {
			select {
			case ch <- ChatChunk{Content: part}:
			case <-ctx.Done():
				return
			}
			select {
			case <-time.After(30 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}
		select {
		case ch <- ChatChunk{Done: true}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func GenerateWithFinalGraph(ctx context.Context, sessionID, query string) (string, error) {
	runnable, err := flow.GetFinalGraph()
	if err != nil {
		return "", err
	}
	ctx = context.WithValue(ctx, "session_id", sessionID)
	messages, err := runnable.Invoke(ctx, flow.FinalGraphRequest{
		Query:     query,
		SessionID: sessionID,
	})
	if err != nil {
		return "", err
	}
	reply := MessagesToReply(messages)
	if reply == "" {
		return "", fmt.Errorf("FinalGraph returned empty reply")
	}
	return reply, nil
}

func MessagesToReply(messages []*schema.Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func splitForPseudoStream(text string, size int) []string {
	if size <= 0 {
		size = 24
	}
	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}
	parts := make([]string, 0, len(runes)/size+1)
	for len(runes) > 0 {
		n := size
		if len(runes) < n {
			n = len(runes)
		}
		parts = append(parts, string(runes[:n]))
		runes = runes[n:]
	}
	return parts
}
