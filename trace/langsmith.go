package trace

import (
	"log"
	"qqqai/config"

	"github.com/cloudwego/eino-ext/callbacks/langsmith"
	"github.com/cloudwego/eino/callbacks"
)

func NewLangSmith() error {
	traceHandler, err := langsmith.NewLangsmithHandler(&langsmith.Config{
		APIKey: config.GlobalConfig.LangSmithConf.APIKey,
		APIURL: config.GlobalConfig.LangSmithConf.APIUrl,
	})
	if err != nil {
		return err
	}
	callbacks.AppendGlobalHandlers(traceHandler)
	log.Println("LangSmith 全局回调已启用")

	return nil
}
