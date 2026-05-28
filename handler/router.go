package handler

import (
	"context"
	"fmt"
	"log"
	"qqqai/adapter"
	"qqqai/ai"
	"qqqai/flow"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gorilla/websocket"
)

// HandleWSMessage 处理 WebSocket 接收到的事件和 AI 引擎
func HandleWSMessage(conn *websocket.Conn, message []byte, botQQ int64, writeTimeout time.Duration, writeMu *sync.Mutex) {
	// 1. 调用 adapter.ParseEvent 解析事件
	event, err := adapter.ParseEvent(message)
	if err != nil {
		log.Printf("解析事件失败: %v", err)
		return
	}

	// 2. 判断 PostType == "message" 且 MessageType == "group"
	if event.PostType != "message" || event.MessageType != "group" {
		log.Printf("忽略非群聊消息事件")
		return
	}

	// 3. 调用 adapter.ExtractCleanText 检查是否被 @
	cleanText, isAt := adapter.ExtractCleanText(event.Message, botQQ)
	if !isAt {
		log.Printf("消息未包含 @ 机器人，忽略处理")
		return
	}

	// 如果被 @，开启 goroutine 处理
	go func() {
		ctx := context.Background()

		// 提取 sessionID
		sessionID := ai.GetSessionID(event.GroupID, event.UserID)

		log.Printf("收到群 %d 用户 %d 的消息: %s", event.GroupID, event.UserID, cleanText)

		// 4. 调用 FinalGraph 获取回复，失败时回退到普通聊天
		reply, err := generateGraphReply(ctx, sessionID, cleanText)
		if err != nil {
			log.Printf("总控图生成回复失败，回退普通聊天: %v", err)
			reply, err = ai.GenerateReply(ctx, sessionID, cleanText)
			if err != nil {
				log.Printf("生成回复失败: %v", err)
				return
			}
		}

		log.Printf("生成回复成功: %s", reply)

		// 5. 调用 adapter.BuildSendGroupMsgAction 构建发送包
		actionData := adapter.BuildSendGroupMsgAction(event.GroupID, reply)
		if actionData == nil {
			log.Printf("构建发送消息动作失败")
			return
		}

		// 通过 conn.WriteMessage 发送回 NapCat
		writeMu.Lock()
		defer writeMu.Unlock()
		if writeTimeout > 0 {
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		}
		err = conn.WriteMessage(websocket.TextMessage, actionData)
		if err != nil {
			log.Printf("发送回复消息失败: %v", err)
			return
		}

		log.Printf("回复消息发送成功 (群 %d)", event.GroupID)
	}()
}

func generateGraphReply(ctx context.Context, sessionID, cleanText string) (string, error) {
	runnable, err := flow.GetFinalGraph()
	if err != nil {
		return "", err
	}

	ctx = context.WithValue(ctx, "session_id", sessionID)
	messages, err := runnable.Invoke(ctx, flow.FinalGraphRequest{
		Query:     cleanText,
		SessionID: sessionID,
	})
	if err != nil {
		return "", err
	}

	reply := messagesToReply(messages)
	if reply == "" {
		return "", fmt.Errorf("总控图返回空回复")
	}
	return reply, nil
}

func messagesToReply(messages []*schema.Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
