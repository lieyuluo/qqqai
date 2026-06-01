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
	event, err := adapter.ParseEvent(message)
	if err != nil {
		log.Printf("解析事件失败: %v", err)
		return
	}

	requestText, sessionID, privateReply, ok := routeEvent(event, botQQ)
	if !ok {
		return
	}

	go func() {
		ctx := context.Background()

		log.Printf("收到用户 %d 的消息: %s", event.UserID, requestText)

		reply, err := generateGraphReply(ctx, sessionID, requestText)
		if err != nil {
			log.Printf("总控图生成回复失败，回退普通聊天: %v", err)
			reply, err = ai.GenerateReply(ctx, sessionID, requestText)
			if err != nil {
				log.Printf("生成回复失败: %v", err)
				return
			}
		}

		log.Printf("生成回复成功: %s", reply)

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
	}()
}

func routeEvent(event *adapter.Event, botQQ int64) (requestText, sessionID string, privateReply, ok bool) {
	if adapter.IsGroupFileEvent(event) {
		requestText = strings.TrimSpace(adapter.FileEventText(event))
		if requestText == "" {
			log.Printf("群文件事件内容为空，忽略处理")
			return "", "", false, false
		}
		return requestText, groupSessionID(event), false, true
	}

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
