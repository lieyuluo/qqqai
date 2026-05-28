package embedding_model

import (
	"context"
	"qqqai/config"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"
)

func initArk() {
	registerEmbeddingModel("ark", func(ctx context.Context) (embedding.Embedder, error) {
		emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
			APIKey: config.GlobalConfig.ArkConf.ArkKey,
			Model:  config.GlobalConfig.ArkConf.ArkEmbeddingModel,
		})
		if err != nil {
			return nil, err
		}
		return emb, nil
	})
}
