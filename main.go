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
			// 将读取到的 byte 移交给 handler.HandleWSMessage 处理
			handler.HandleWSMessage(conn, message, config.GetBotQQ(), writeTimeout, writeMu)
		}
	}
}

func initBotDataPipeline(ctx context.Context) error {
	var err error
	if db.Milvus == nil {
		db.Milvus, err = db.NewMilvus(ctx)
		if err != nil {
			return err
		}
	}

	if document.Loader == nil {
		document.Loader, err = document.NewLoader(ctx)
		if err != nil {
			return err
		}
	}

	if document.Parser == nil {
		document.Parser, err = document.NewParser(ctx)
		if err != nil {
			return err
		}
	}

	if document.Splitter == nil {
		document.Splitter, err = document.NewSplitter(ctx)
		if err != nil {
			return err
		}
	}

	indexer.NewIndexer()
	retriever.NewRetriever()

	if err := rag_flow.InitIndexingGraph(ctx); err != nil {
		return err
	}

	taskModel, err := chat_model.GetChatModel(ctx, config.GlobalConfig.ChatModelType)
	if err != nil {
		return err
	}

	if err := flow.InitRAGChatFlow(ctx, memory.NewMemoryStore(), taskModel); err != nil {
		return err
	}

	if err := sql_tools.InitMCPTools(ctx); err != nil {
		return err
	}

	if err := flow.InitFinalGraph(ctx, storage.NewRedisCheckPointStore()); err != nil {
		return err
	}

	return nil
}

func main() {
	configPath := ".env"
	_, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	ctx := context.Background()
	if err := initBotDataPipeline(ctx); err != nil {
		log.Fatalf("初始化 QQBot 数据处理流程失败: %v", err)
	}

	// 初始化聊天模型
	err = ai.InitChatModel()
	if err != nil {
		log.Fatalf("初始化聊天模型失败: %v", err)
	}

	// 注册路由
	http.HandleFunc("/ws", wsEndpoint)

	// 启动服务器
	port := config.GetPort()
	log.Printf("机器人 QQ 号: %d", config.GetBotQQ())

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("启动 HTTP 服务器失败: %v", err)
	}
}
