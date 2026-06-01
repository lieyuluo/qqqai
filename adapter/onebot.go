package adapter

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Event 表示 OneBot V11 的事件结构
type Event struct {
	PostType    string      `json:"post_type"`
	NoticeType  string      `json:"notice_type"`
	SubType     string      `json:"sub_type"`
	MessageType string      `json:"message_type"`
	MessageID   int64       `json:"message_id"`
	GroupID     int64       `json:"group_id"`
	UserID      int64       `json:"user_id"`
	Message     string      `json:"message"`
	RawMessage  string      `json:"raw_message"`
	SelfID      int64       `json:"self_id"`
	File        *FileInfo   `json:"file"`
	Reply       ReplyAction `json:"reply"`
}

type FileInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	BusID int64  `json:"busid"`
	URL   string `json:"url"`
}

// ReplyAction 表示回复动作结构
type ReplyAction struct {
	Async bool `json:"async"`
}

// Action 表示发送消息的动作
type Action struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

// ParseEvent 将 WebSocket 收到的 byte 数组解析为标准的 Event 结构体
func ParseEvent(rawData []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(rawData, &event); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %v", err)
	}

	if !IsSupportedEvent(&event) {
		return nil, fmt.Errorf("忽略不支持的事件: post_type=%s message_type=%s notice_type=%s sub_type=%s", event.PostType, event.MessageType, event.NoticeType, event.SubType)
	}

	return &event, nil
}

func IsSupportedEvent(event *Event) bool {
	if event == nil {
		return false
	}
	return IsSupportedMessage(event) || IsGroupFileEvent(event)
}

func IsSupportedMessage(event *Event) bool {
	return event != nil && event.PostType == "message" && (event.MessageType == "group" || event.MessageType == "private")
}

func IsGroupFileEvent(event *Event) bool {
	return event != nil && event.PostType == "notice" && event.NoticeType == "group_upload" && event.GroupID != 0
}

func BuildSendGroupMsgAction(groupID int64, text string) []byte {
	action := Action{
		Action: "send_group_msg",
		Params: map[string]any{
			"group_id": groupID,
			"message":  text,
		},
	}

	data, err := json.Marshal(action)
	if err != nil {
		log.Printf("构造发送群消息动作失败: %v", err)
		return nil
	}

	return data
}

func BuildSendPrivateMsgAction(userID int64, text string) []byte {
	action := Action{
		Action: "send_private_msg",
		Params: map[string]any{
			"user_id": userID,
			"message": text,
		},
	}

	data, err := json.Marshal(action)
	if err != nil {
		log.Printf("构造发送私聊消息动作失败: %v", err)
		return nil
	}

	return data
}

// ExtractCleanText 判断消息中是否包含 [CQ:at,qq={botQQ}]，剥离该 CQ 码并返回清理后的文本
func ExtractCleanText(rawMsg string, botQQ int64) (string, bool) {
	atCode := fmt.Sprintf("[CQ:at,qq=%d]", botQQ)
	isAt := strings.Contains(rawMsg, atCode)
	if isAt {
		cleanText := strings.ReplaceAll(rawMsg, atCode, "")
		cleanText = strings.TrimSpace(cleanText)
		return cleanText, true
	}

	return rawMsg, false
}

func FileEventText(event *Event) string {
	if event == nil || event.File == nil {
		return ""
	}

	parts := []string{fmt.Sprintf("群文件上传: %s", strings.TrimSpace(event.File.Name))}
	if event.File.URL != "" {
		parts = append(parts, fmt.Sprintf("文件地址: %s", event.File.URL))
	}
	if event.File.ID != "" {
		parts = append(parts, fmt.Sprintf("文件ID: %s", event.File.ID))
	}
	if event.File.Size > 0 {
		parts = append(parts, fmt.Sprintf("文件大小: %d 字节", event.File.Size))
	}
	return strings.Join(parts, "\n")
}
