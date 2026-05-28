package chat_model

import (
	"context"
	"qqqai/config"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

func initOpenAI() {
	registerChatModel("openai", func(ctx context.Context) (model.BaseChatModel, error) {
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  config.GetAPIKey(),
			Model:   config.GetModelName(),
			BaseURL: config.GetBaseURL(),
		})
	})
}
