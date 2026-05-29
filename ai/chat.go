package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"qqqai/config"
	"qqqai/model/chat_model"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var (
	chatModel model.BaseChatModel
	chatMu    sync.RWMutex
	sessions  sync.Map
)

func InitChatModel(apiKey, baseURL, modelName string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API Key 不能为空")
	}
	if strings.TrimSpace(baseURL) == "" {
		return fmt.Errorf("API Base URL 不能为空")
	}
	if strings.TrimSpace(modelName) == "" {
		return fmt.Errorf("模型名称不能为空")
	}

	cm, err := chat_model.GetChatModel(context.Background(), config.GlobalConfig.ChatModelType)
	if err != nil {
		return err
	}

	chatMu.Lock()
	chatModel = cm
	chatMu.Unlock()
	return nil
}

func GetSessionID(groupID, userID int64) string {
	return fmt.Sprintf("group:%d:user:%d", groupID, userID)
}

func GenerateReply(ctx context.Context, sessionID, userText string) (string, error) {
	chatMu.RLock()
	cm := chatModel
	chatMu.RUnlock()
	if cm == nil {
		return "", fmt.Errorf("聊天模型未初始化")
	}

	text := strings.TrimSpace(userText)
	if text == "" {
		return "", fmt.Errorf("消息内容为空")
	}

	history := getHistory(sessionID)
	messages := make([]*schema.Message, 0, len(history)+2)
	if persona := strings.TrimSpace(config.GetPersona()); persona != "" {
		messages = append(messages, schema.SystemMessage(persona))
	}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(text))

	reply, err := cm.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	content := strings.TrimSpace(reply.Content)
	if content == "" {
		return "", fmt.Errorf("模型返回空回复")
	}

	saveTurn(sessionID, schema.UserMessage(text), schema.AssistantMessage(content, nil))
	return content, nil
}

func getHistory(sessionID string) []*schema.Message {
	value, ok := sessions.Load(sessionID)
	if !ok {
		return nil
	}
	history := value.([]*schema.Message)
	copied := make([]*schema.Message, len(history))
	copy(copied, history)
	return copied
}

func saveTurn(sessionID string, userMsg, assistantMsg *schema.Message) {
	history := append(getHistory(sessionID), userMsg, assistantMsg)
	maxMessages := config.GetMaxMessages()
	if maxMessages > 0 && len(history) > maxMessages {
		history = history[len(history)-maxMessages:]
	}
	sessions.Store(sessionID, history)
}
