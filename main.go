package main

import (
	"context"
	"log"
	"net/http"
	"qqqai/ai"
	"qqqai/config"
	"qqqai/flow"
	"qqqai/handler"
	"qqqai/model/chat_model"
	"qqqai/rag/rag_flow"
	"qqqai/rag/rag_tools/db"
	"qqqai/rag/rag_tools/indexer"
	"qqqai/rag/rag_tools/retriever"
	"qqqai/tool/document"
	"qqqai/tool/memory"
	"qqqai/tool/sql_tools"
	"qqqai/tool/storage"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 全局 WebSocket 升级器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 从配置获取允许的 Origin 列表
		allowedOrigins := config.GetAllowedOrigins()
		origin := r.Header.Get("Origin")

		// 如果允许所有来源
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				return true
			}
		}

		return false
	},
}

// wsEndpoint 处理 WebSocket upgrade，并在 for 循环中读取消息
func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket 连接建立: %s", r.RemoteAddr)

	readTimeout := time.Duration(config.GetReadTimeout()) * time.Second
	writeTimeout := time.Duration(config.GetWriteTimeout()) * time.Second
	writeMu := &sync.Mutex{}

	// 在 for 循环中读取消息
	for {
		if readTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(readTimeout))
		}

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// 如果是连接关闭错误，正常退出
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("读取消息错误: %v", err)
			}
			break
		}

		// 只处理文本消息
		if messageType == websocket.TextMessage {
			log.Printf("收到消息: %s", string(message))

			// 将读取到的 byte 移交给 handler.HandleWSMessage 处理
			handler.HandleWSMessage(conn, message, config.GetBotQQ(), writeTimeout, writeMu)
		}
	}
}

func initBotDataPipeline(ctx context.Context) error {
	var err error
	if db.Milvus == nil {
		log.Printf("开始初始化 Milvus 客户端: %s", config.GlobalConfig.MilvusConf.MilvusAddr)
		db.Milvus, err = db.NewMilvus(ctx)
		if err != nil {
			return err
		}
		log.Printf("Milvus 客户端初始化成功")
	}

	if document.Loader == nil {
		log.Printf("开始初始化 Document Loader")
		document.Loader, err = document.NewLoader(ctx)
		if err != nil {
			return err
		}
		log.Printf("Document Loader 初始化成功")
	}

	if document.Parser == nil {
		log.Printf("开始初始化 Document Parser")
		document.Parser, err = document.NewParser(ctx)
		if err != nil {
			return err
		}
		log.Printf("Document Parser 初始化成功")
	}

	if document.Splitter == nil {
		log.Printf("开始初始化 Document Splitter")
		document.Splitter, err = document.NewSplitter(ctx)
		if err != nil {
			return err
		}
		log.Printf("Document Splitter 初始化成功")
	}

	log.Printf("开始注册 RAG Indexer/Retriever")
	indexer.NewIndexer()
	retriever.NewRetriever()
	log.Printf("RAG Indexer/Retriever 注册成功")

	log.Printf("开始初始化 IndexingGraph")
	if err := rag_flow.InitIndexingGraph(ctx); err != nil {
		return err
	}
	log.Printf("IndexingGraph 初始化成功")

	log.Printf("开始初始化 RAGChatFlow 任务模型: %s", config.GlobalConfig.ChatModelType)
	taskModel, err := chat_model.GetChatModel(ctx, config.GlobalConfig.ChatModelType)
	if err != nil {
		return err
	}

	log.Printf("开始初始化 RAGChatFlow")
	if err := flow.InitRAGChatFlow(ctx, memory.NewMemoryStore(), taskModel); err != nil {
		return err
	}
	log.Printf("RAGChatFlow 初始化成功")

	log.Printf("开始初始化 MCPTools")
	if err := sql_tools.InitMCPTools(ctx); err != nil {
		return err
	}
	log.Printf("MCPTools 初始化成功")

	log.Printf("开始初始化 FinalGraph")
	if err := flow.InitFinalGraph(ctx, storage.NewRedisCheckPointStore()); err != nil {
		return err
	}
	log.Printf("FinalGraph 初始化成功")

	return nil
}

func main() {
	configPath := ".env"
	_, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功: %s", configPath)

	ctx := context.Background()
	if err := initBotDataPipeline(ctx); err != nil {
		log.Fatalf("初始化 QQBot 数据处理流程失败: %v", err)
	}

	// 初始化聊天模型
	err = ai.InitChatModel(
		config.GetAPIKey(),
		config.GetBaseURL(),
		config.GetModelName(),
	)
	if err != nil {
		log.Fatalf("初始化聊天模型失败: %v", err)
	}

	// 注册路由
	http.HandleFunc("/ws", wsEndpoint)

	// 启动服务器
	port := config.GetPort()
	log.Printf("服务器启动，监听端口: %s", port)
	log.Printf("机器人 QQ 号: %d", config.GetBotQQ())
	log.Printf("使用模型: %s", config.GetModelName())
	log.Printf("AI 人设: %s", config.GetPersona())
	log.Printf("最大会话消息数: %d", config.GetMaxMessages())

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("启动 HTTP 服务器失败: %v", err)
	}
}
