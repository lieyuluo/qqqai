package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config 结构体定义所有配置项
type Config struct {
	Bot                BotConfig           `yaml:"bot"`
	LLM                LLMConfig           `yaml:"llm"`
	Session            SessionConfig       `yaml:"session"`
	Websocket          WebsocketConfig     `yaml:"websocket"`
	Memory             MemoryConfig        `yaml:"memory"`
	ChatModelType      string              `yaml:"chat_model_type"`
	IntentModelType    string              `yaml:"intent_model_type"`
	EmbeddingModelType string              `yaml:"embedding_model_type"`
	ArkConf            ArkConfig           `yaml:"ark"`
	DeepSeekConf       DeepSeekConfig      `yaml:"deepseek"`
	QwenConf           QwenConfig          `yaml:"qwen"`
	MySQLConf          MySQLConfig         `yaml:"mysql"`
	RedisConf          RedisConfig         `yaml:"redis"`
	LangSmithConf      LangSmithConfig     `yaml:"langsmith"`
	MilvusConf         MilvusConfig        `yaml:"milvus"`
	ESConf             ElasticsearchConfig `yaml:"elasticsearch"`
}

// BotConfig 机器人相关配置
type BotConfig struct {
	QQ   int64  `yaml:"qq"`
	Port string `yaml:"port"`
}

// LLMConfig 大模型相关配置
type LLMConfig struct {
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	ModelName string `yaml:"model_name"`
	Persona   string `yaml:"persona"`
}

type ArkConfig struct {
	ArkKey            string `yaml:"ark_key"`
	ArkChatModel      string `yaml:"ark_chat_model"`
	ArkEmbeddingModel string `yaml:"ark_embedding_model"`
}

type DeepSeekConfig struct {
	DeepSeekKey       string `yaml:"deepseek_key"`
	DeepSeekChatModel string `yaml:"deepseek_chat_model"`
	BaseUrl           string `yaml:"base_url"`
}

type QwenConfig struct {
	QwenKey       string `yaml:"qwen_key"`
	QwenEmbedding string `yaml:"qwen_embedding"`
	BaseUrl       string `yaml:"base_url"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
}

type LangSmithConfig struct {
	APIKey string `yaml:"api_key"`
	APIUrl string `yaml:"api_url"`
}

type MilvusConfig struct {
	MilvusAddr     string `yaml:"milvus_addr"`
	MilvusUserName string `yaml:"milvus_username"`
	MilvusPassword string `yaml:"milvus_password"`
	CollectionName string `yaml:"collection_name"`
	TopK           string `yaml:"top_k"`
}

type ElasticsearchConfig struct {
	Addresses []string `yaml:"addresses"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Index     string   `yaml:"index"`
}

// SessionConfig 会话相关配置
type SessionConfig struct {
	MaxMessages int `yaml:"max_messages"`
}

// WebsocketConfig WebSocket 相关配置
type WebsocketConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	ReadTimeout    int      `yaml:"read_timeout"`
	WriteTimeout   int      `yaml:"write_timeout"`
}

// MemoryConfig 记忆相关配置
type MemoryConfig struct {
	Dir             string `yaml:"dir"`
	SummaryInterval int    `yaml:"summary_interval"`
	MaxEntries      int    `yaml:"max_entries"`
}

var (
	// 全局配置实例
	GlobalConfig *Config
	Cfg          *Config
)

// LoadConfig loads .env and environment variables.
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if err := godotenv.Load(configPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取环境配置失败: %v", err)
	}

	applyDefaults(config)
	applyEnvOverrides(config)

	// 验证配置
	if config.Bot.QQ == 0 {
		return nil, fmt.Errorf("机器人QQ号不能为空")
	}
	if isBlank(config.LLM.APIKey) || config.LLM.APIKey == "your-api-key" {
		return nil, fmt.Errorf("API Key 不能为空或占位值")
	}
	if isBlank(config.LLM.BaseURL) {
		return nil, fmt.Errorf("API Base URL 不能为空")
	}
	if isBlank(config.LLM.ModelName) || config.LLM.ModelName == "your-model-name" {
		return nil, fmt.Errorf("模型名称不能为空或占位值")
	}

	// 设置全局配置
	GlobalConfig = config
	Cfg = config

	return config, nil
}

func applyEnvOverrides(config *Config) {
	setString := func(value *string, envName string) {
		if envValue := os.Getenv(envName); envValue != "" {
			*value = envValue
		}
	}
	setInt := func(value *int, envName string) {
		if envValue := os.Getenv(envName); envValue != "" {
			if parsed, err := strconv.Atoi(envValue); err == nil {
				*value = parsed
			}
		}
	}
	setInt64 := func(value *int64, envName string) {
		if envValue := os.Getenv(envName); envValue != "" {
			if parsed, err := strconv.ParseInt(envValue, 10, 64); err == nil {
				*value = parsed
			}
		}
	}

	setInt64(&config.Bot.QQ, "BOT_QQ")
	setString(&config.Bot.Port, "BOT_PORT")
	setString(&config.LLM.APIKey, "LLM_API_KEY")
	setString(&config.LLM.BaseURL, "LLM_BASE_URL")
	setString(&config.LLM.ModelName, "LLM_MODEL_NAME")
	setString(&config.LLM.Persona, "LLM_PERSONA")
	config.LLM.Persona = strings.ReplaceAll(config.LLM.Persona, `\n`, "\n")
	setString(&config.ChatModelType, "CHAT_MODEL_TYPE")
	setString(&config.IntentModelType, "INTENT_MODEL_TYPE")
	setString(&config.EmbeddingModelType, "EMBEDDING_MODEL_TYPE")
	setString(&config.ArkConf.ArkKey, "ARK_KEY")
	setString(&config.ArkConf.ArkChatModel, "ARK_CHAT_MODEL")
	setString(&config.ArkConf.ArkEmbeddingModel, "ARK_EMBEDDING_MODEL")
	setString(&config.DeepSeekConf.DeepSeekKey, "DEEPSEEK_KEY")
	setString(&config.DeepSeekConf.DeepSeekChatModel, "DEEPSEEK_CHAT_MODEL")
	setString(&config.DeepSeekConf.BaseUrl, "DEEPSEEK_BASE_URL")
	setString(&config.QwenConf.QwenKey, "QWEN_KEY")
	setString(&config.QwenConf.QwenEmbedding, "QWEN_EMBEDDING")
	setString(&config.QwenConf.BaseUrl, "QWEN_BASE_URL")
	setString(&config.MySQLConf.Host, "MYSQL_HOST")
	setString(&config.MySQLConf.Port, "MYSQL_PORT")
	setString(&config.MySQLConf.Username, "MYSQL_USERNAME")
	setString(&config.MySQLConf.Password, "MYSQL_PASSWORD")
	setString(&config.MySQLConf.Database, "MYSQL_DATABASE")
	setString(&config.RedisConf.Addr, "REDIS_ADDR")
	setString(&config.RedisConf.Password, "REDIS_PASSWORD")
	setString(&config.RedisConf.DB, "REDIS_DB")
	setString(&config.LangSmithConf.APIKey, "LANGSMITH_API_KEY")
	setString(&config.LangSmithConf.APIUrl, "LANGSMITH_API_URL")
	setString(&config.MilvusConf.MilvusAddr, "MILVUS_ADDR")
	setString(&config.MilvusConf.MilvusUserName, "MILVUS_USERNAME")
	setString(&config.MilvusConf.MilvusPassword, "MILVUS_PASSWORD")
	setString(&config.MilvusConf.CollectionName, "MILVUS_COLLECTION_NAME")
	setString(&config.MilvusConf.TopK, "MILVUS_TOP_K")
	setString(&config.ESConf.Username, "ELASTICSEARCH_USERNAME")
	setString(&config.ESConf.Password, "ELASTICSEARCH_PASSWORD")
	setString(&config.ESConf.Index, "ELASTICSEARCH_INDEX")
	setString(&config.Memory.Dir, "MEMORY_DIR")

	if addresses := os.Getenv("ELASTICSEARCH_ADDRESSES"); addresses != "" {
		config.ESConf.Addresses = splitComma(addresses)
	}
	if allowedOrigins := os.Getenv("WEBSOCKET_ALLOWED_ORIGINS"); allowedOrigins != "" {
		config.Websocket.AllowedOrigins = splitComma(allowedOrigins)
	}
	setInt(&config.Session.MaxMessages, "SESSION_MAX_MESSAGES")
	setInt(&config.Memory.SummaryInterval, "MEMORY_SUMMARY_INTERVAL")
	setInt(&config.Memory.MaxEntries, "MEMORY_MAX_ENTRIES")
	setInt(&config.Websocket.ReadTimeout, "WEBSOCKET_READ_TIMEOUT")
	setInt(&config.Websocket.WriteTimeout, "WEBSOCKET_WRITE_TIMEOUT")
}

func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func applyDefaults(config *Config) {
	if config.Bot.Port == "" {
		config.Bot.Port = "8080"
	}
	if config.Session.MaxMessages == 0 {
		config.Session.MaxMessages = 10
	}
	if config.Websocket.ReadTimeout == 0 {
		config.Websocket.ReadTimeout = 300
	}
	if config.Websocket.WriteTimeout == 0 {
		config.Websocket.WriteTimeout = 10
	}
	if len(config.Websocket.AllowedOrigins) == 0 {
		config.Websocket.AllowedOrigins = []string{"*"}
	}
	if config.Memory.Dir == "" {
		config.Memory.Dir = "data/memory"
	}
	if config.Memory.SummaryInterval == 0 {
		config.Memory.SummaryInterval = 20
	}
	if config.Memory.MaxEntries == 0 {
		config.Memory.MaxEntries = 20
	}
	if config.ChatModelType == "" {
		config.ChatModelType = "openai"
	}
	if config.IntentModelType == "" {
		config.IntentModelType = config.ChatModelType
	}
	if config.EmbeddingModelType == "" {
		config.EmbeddingModelType = "qwen"
	}
	if config.DeepSeekConf.BaseUrl == "" {
		config.DeepSeekConf.BaseUrl = "https://api.deepseek.com"
	}
	if config.QwenConf.BaseUrl == "" {
		config.QwenConf.BaseUrl = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if config.MilvusConf.TopK == "" {
		config.MilvusConf.TopK = "5"
	}
	if config.RedisConf.Addr == "" {
		config.RedisConf.Addr = "localhost:6379"
	}
	if config.RedisConf.DB == "" {
		config.RedisConf.DB = "0"
	}
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

// GetBotQQ 获取机器人QQ号
func GetBotQQ() int64 {
	if GlobalConfig == nil {
		return 0
	}
	return GlobalConfig.Bot.QQ
}

// GetPort 获取服务监听端口
func GetPort() string {
	if GlobalConfig == nil || GlobalConfig.Bot.Port == "" {
		return ":8080"
	}
	if strings.Contains(GlobalConfig.Bot.Port, ":") {
		return GlobalConfig.Bot.Port
	}
	return ":" + GlobalConfig.Bot.Port
}

// GetAPIKey 获取API Key
func GetAPIKey() string {
	if GlobalConfig == nil {
		return ""
	}
	return GlobalConfig.LLM.APIKey
}

// GetBaseURL 获取API Base URL
func GetBaseURL() string {
	if GlobalConfig == nil {
		return ""
	}
	return GlobalConfig.LLM.BaseURL
}

// GetModelName 获取模型名称
func GetModelName() string {
	if GlobalConfig == nil {
		return ""
	}
	return GlobalConfig.LLM.ModelName
}

// GetPersona 获取AI人设提示词
func GetPersona() string {
	if GlobalConfig == nil {
		return ""
	}
	return GlobalConfig.LLM.Persona
}

// GetMaxMessages 获取会话最大消息数量
func GetMaxMessages() int {
	if GlobalConfig == nil || GlobalConfig.Session.MaxMessages <= 0 {
		return 10
	}
	return GlobalConfig.Session.MaxMessages
}

// GetAllowedOrigins 获取允许的Origin列表
func GetAllowedOrigins() []string {
	if GlobalConfig == nil || len(GlobalConfig.Websocket.AllowedOrigins) == 0 {
		return []string{"*"}
	}
	return GlobalConfig.Websocket.AllowedOrigins
}

// GetReadTimeout 获取读取消息超时时间
func GetReadTimeout() int {
	if GlobalConfig == nil || GlobalConfig.Websocket.ReadTimeout <= 0 {
		return 10
	}
	return GlobalConfig.Websocket.ReadTimeout
}

// GetWriteTimeout 获取写入消息超时时间
func GetWriteTimeout() int {
	if GlobalConfig == nil || GlobalConfig.Websocket.WriteTimeout <= 0 {
		return 10
	}
	return GlobalConfig.Websocket.WriteTimeout
}
