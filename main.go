package main

import (
	"context"
	"log"
	"qqqai/ai"
	"qqqai/config"
	"qqqai/flow"
	"qqqai/handler"
	"qqqai/internal/controller"
	"qqqai/internal/dao"
	"qqqai/internal/service"
	localstorage "qqqai/internal/storage"
	"qqqai/internal/worker"
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

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gorilla/websocket"
)

func serveWSConn(conn *websocket.Conn) {
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

// wsEndpoint keeps the original /ws OneBot route semantics on GoFrame.
func wsEndpoint(r *ghttp.Request) {
	ws, err := r.WebSocket()
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer ws.Close()
	serveWSConn(ws.Conn)
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

	if err := ai.InitChatModel(); err != nil {
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

	if err := dao.Init(ctx); err != nil {
		log.Fatalf("初始化 MySQL 连接失败: %v", err)
	}
	defer dao.Close()

	if err := service.EnsureAdmin(ctx); err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}
	if err := controller.EnsureUploadDir(config.GetUploadDir()); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	chatService := service.NewChatService()
	chatPool := worker.NewChatTaskPool(chatService, config.GetChatWorkerCount(), config.GetChatQueueSize())
	defer chatPool.Close()
	handler.SetChatTaskPool(chatPool)

	fileWorker := worker.NewFileIndexWorker(appCtx, config.GetFileIndexWorkerCount(), config.GetFileIndexQueueSize())
	defer fileWorker.Close()

	app := controller.NewApp(
		service.NewAuthService(),
		chatService,
		service.NewSQLService(),
		chatPool,
		fileWorker,
		localstorage.NewLocalStorage(config.GetUploadDir()),
	)

	server := g.Server()
	server.SetAddr(config.GetPort())
	server.SetDumpRouterMap(false)
	controller.RegisterRoutes(server, app, wsEndpoint)

	port := config.GetPort()
	log.Printf("机器人 QQ 号: %d", config.GetBotQQ())
	log.Printf("GoFrame Server listening on %s", port)
	server.Run()
}
