//go:build unsupported

package chat_model

import (
	"context"
	"qqqai/config"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
)

func initDeepSeek() {
	registerChatModel("deepseek", func(ctx context.Context) (model.BaseChatModel, error) {
		return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey:  config.GlobalConfig.DeepSeekConf.DeepSeekKey,
			Model:   config.GlobalConfig.DeepSeekConf.DeepSeekChatModel,
			BaseURL: config.GlobalConfig.DeepSeekConf.BaseUrl,
		})
	})
}
