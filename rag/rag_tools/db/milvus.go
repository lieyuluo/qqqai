package db

import (
	"context"
	"qqqai/config"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

var Milvus client.Client

func NewMilvus(ctx context.Context) (client.Client, error) {
	cli, err := client.NewClient(ctx, client.Config{
		Address:  config.GlobalConfig.MilvusConf.MilvusAddr,
		Username: config.GlobalConfig.MilvusConf.MilvusUserName,
		Password: config.GlobalConfig.MilvusConf.MilvusPassword,
	})
	if err != nil {
		return nil, err
	}

	return cli, nil
}
