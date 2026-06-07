package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"qqqai/adapter"
	"qqqai/ai"
	"qqqai/config"
	"qqqai/internal/service"
	"qqqai/internal/worker"
	"qqqai/rag/rag_flow"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/document"
	"github.com/gorilla/websocket"
)

const napCatAccessToken = "yAEsax.~ofNwpFDk"

var chatTaskPool *worker.ChatTaskPool

func SetChatTaskPool(pool *worker.ChatTaskPool) {
	chatTaskPool = pool
}

// HandleWSMessage 处理 WebSocket 接收到的事件和 AI 引擎
func HandleWSMessage(conn *websocket.Conn, message []byte, botQQ int64, writeTimeout time.Duration, writeMu *sync.Mutex) {
	event, err := adapter.ParseEvent(message)
	if err != nil {
		log.Printf("解析事件失败: %v", err)
		return
	}
	if adapter.IsGroupFileEvent(event) {
		handleGroupFileEvent(conn, event, writeTimeout, writeMu)
		return
	}

	requestText, sessionID, privateReply, ok := routeEvent(event, botQQ)
	if !ok {
		return
	}

	go func() {
		ctx := context.Background()

		log.Printf("收到用户 %d 的消息: %s", event.UserID, requestText)

		reply, err := generateChatReply(ctx, sessionID, requestText)
		if err != nil {
			log.Printf("生成回复失败: %v", err)
			return
		}

		log.Printf("生成回复成功: %s", reply)

		sendReply(conn, event, reply, privateReply, writeTimeout, writeMu)
	}()
}

func routeEvent(event *adapter.Event, botQQ int64) (requestText, sessionID string, privateReply, ok bool) {
	if !adapter.IsSupportedMessage(event) {
		log.Printf("忽略不支持的消息事件")
		return "", "", false, false
	}

	rawText := event.RawMessage
	if rawText == "" {
		rawText = event.Message
	}

	switch event.MessageType {
	case "group":
		cleanText, isAt := adapter.ExtractCleanText(rawText, botQQ)
		if !isAt {
			log.Printf("群消息未包含 @ 机器人，忽略处理")
			return "", "", false, false
		}
		cleanText = strings.TrimSpace(cleanText)
		if cleanText == "" {
			log.Printf("群消息内容为空，忽略处理")
			return "", "", false, false
		}
		return cleanText, groupSessionID(event), false, true
	case "private":
		cleanText := strings.TrimSpace(rawText)
		if cleanText == "" {
			log.Printf("私聊消息内容为空，忽略处理")
			return "", "", false, false
		}
		return cleanText, privateSessionID(event), true, true
	default:
		log.Printf("忽略未知消息类型: %s", event.MessageType)
		return "", "", false, false
	}
}

func groupSessionID(event *adapter.Event) string {
	return ai.GetSessionID(event.GroupID, event.UserID)
}

func privateSessionID(event *adapter.Event) string {
	return fmt.Sprintf("private:%d", event.UserID)
}

func buildReplyAction(event *adapter.Event, reply string, privateReply bool) []byte {
	if privateReply {
		return adapter.BuildSendPrivateMsgAction(event.UserID, reply)
	}
	return adapter.BuildSendGroupMsgAction(event.GroupID, reply)
}

func handleGroupFileEvent(conn *websocket.Conn, event *adapter.Event, writeTimeout time.Duration, writeMu *sync.Mutex) {
	go func() {
		ctx := context.Background()
		reply := indexUploadedGroupFile(ctx, event)
		sendReply(conn, event, reply, false, writeTimeout, writeMu)
	}()
}

func indexUploadedGroupFile(ctx context.Context, event *adapter.Event) string {
	if event == nil || event.File == nil {
		log.Printf("群文件事件缺少文件信息，忽略处理")
		return "收到群文件事件，但缺少文件信息，无法索引。"
	}

	fileName := strings.TrimSpace(event.File.Name)
	if fileName == "" {
		fileName = "未命名文件"
	}
	fileID := strings.TrimSpace(event.File.ID)
	if fileID == "" || event.File.BusID == 0 {
		log.Printf("群文件 %s 缺少 file_id 或 busid，无法获取下载地址", fileName)
		return fmt.Sprintf("收到群文件 %s，但事件中缺少文件标识，无法自动索引。", fileName)
	}

	fileURL, err := adapter.FetchGroupFileURL(ctx, config.GetNapCatHTTPBaseURL(), napCatAccessToken, event.GroupID, fileID, event.File.BusID)
	if err != nil {
		log.Printf("获取群文件 %s 下载地址失败: %v", fileName, err)
		return fmt.Sprintf("收到群文件 %s，但获取下载地址失败: %v", fileName, err)
	}

	path, err := downloadGroupFile(ctx, fileName, fileURL, event.File.Size)
	if err != nil {
		log.Printf("下载群文件 %s 失败: %v", fileName, err)
		return fmt.Sprintf("群文件 %s 下载失败: %v", fileName, err)
	}
	defer os.Remove(path)

	runnable, err := rag_flow.GetIndexingGraph()
	if err != nil {
		log.Printf("获取 RAG 索引图失败: %v", err)
		return fmt.Sprintf("群文件 %s 已下载，但索引图未初始化，暂时无法入库。", fileName)
	}

	ids, err := runnable.Invoke(ctx, document.Source{URI: path})
	if err != nil {
		log.Printf("索引群文件 %s 失败: %v", fileName, err)
		return fmt.Sprintf("群文件 %s 索引失败: %v", fileName, err)
	}

	log.Printf("群文件 %s 索引完成，写入 %d 条记录", fileName, len(ids))
	return fmt.Sprintf("群文件 %s 已完成索引，写入 %d 条记录。", fileName, len(ids))
}

func downloadGroupFile(ctx context.Context, fileName, fileURL string, fileSize int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	adapter.SetBearerAuthHeader(req, napCatAccessToken)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("下载地址返回状态码 %d", resp.StatusCode)
	}

	ext := filepath.Ext(fileName)
	tmp, err := os.CreateTemp("", "qqqai-upload-*"+ext)
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	success := false
	defer func() {
		tmp.Close()
		if !success {
			os.Remove(path)
		}
	}()

	reader := resp.Body
	if fileSize > 0 {
		reader = io.NopCloser(io.LimitReader(resp.Body, fileSize+1))
	}
	written, err := io.Copy(tmp, reader)
	if err != nil {
		return "", err
	}
	if fileSize > 0 && written > fileSize {
		return "", fmt.Errorf("下载内容超过事件声明大小")
	}

	success = true
	return path, nil
}

func sendReply(conn *websocket.Conn, event *adapter.Event, reply string, privateReply bool, writeTimeout time.Duration, writeMu *sync.Mutex) {
	actionData := buildReplyAction(event, reply, privateReply)
	if actionData == nil {
		log.Printf("构建发送消息动作失败")
		return
	}

	writeMu.Lock()
	defer writeMu.Unlock()
	if writeTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	}
	if err := conn.WriteMessage(websocket.TextMessage, actionData); err != nil {
		log.Printf("发送回复消息失败: %v", err)
		return
	}

	log.Printf("回复消息发送成功")
}

func generateChatReply(ctx context.Context, sessionID, cleanText string) (string, error) {
	req := service.ChatRequest{SessionID: sessionID, Query: cleanText}
	if chatTaskPool != nil {
		resp, err := chatTaskPool.Submit(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Reply, nil
	}
	resp, err := service.NewChatService().Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Reply, nil
}
