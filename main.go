package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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
	"qqqai/tool/groupfile"
	"qqqai/tool/memory"
	"qqqai/tool/sql_tools"
	"qqqai/tool/storage"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var ready atomic.Bool

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

func initIndexingPipeline(ctx context.Context) error {
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

	if err := rag_flow.InitIndexingGraph(ctx); err != nil {
		return err
	}

	return nil
}

func initBotDataPipeline(ctx context.Context) error {
	if err := initIndexingPipeline(ctx); err != nil {
		return err
	}

	retriever.NewRetriever()

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

	if err := ai.InitChatModel(); err != nil {
		return err
	}

	if err := storage.InitRedis(ctx); err != nil {
		return err
	}

	if err := flow.InitFinalGraph(ctx, storage.NewRedisCheckPointStore()); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configPath := ".env"
	_, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	ctx := context.Background()
	switch appMode() {
	case "server":
		return runServer(ctx)
	case "indexer":
		return runIndexer(ctx)
	default:
		return fmt.Errorf("未知启动模式 %q，可用模式: server, indexer", appMode())
	}
}

func appMode() string {
	if mode := os.Getenv("QQQAI_MODE"); mode != "" {
		return mode
	}
	if mode := os.Getenv("APP_MODE"); mode != "" {
		return mode
	}
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return "server"
}

func runServer(ctx context.Context) error {
	if err := initBotDataPipeline(ctx); err != nil {
		return fmt.Errorf("初始化 QQBot 数据处理流程失败: %w", err)
	}
	ready.Store(true)

	mux := http.NewServeMux()
	registerProbeHandlers(mux)
	mux.HandleFunc("/ws", wsEndpoint)

	port := config.GetPort()
	log.Printf("机器人 QQ 号: %d", config.GetBotQQ())
	log.Printf("qqqai server listening on %s", port)
	return http.ListenAndServe(port, mux)
}

func runIndexer(ctx context.Context) error {
	if err := initIndexingPipeline(ctx); err != nil {
		return fmt.Errorf("初始化群文件索引流程失败: %w", err)
	}
	ready.Store(true)

	mux := http.NewServeMux()
	registerProbeHandlers(mux)
	groupfile.RegisterHandlers(mux)

	port := config.GetPort()
	log.Printf("qqqai indexer listening on %s", port)
	return http.ListenAndServe(port, mux)
}

func registerProbeHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
}
