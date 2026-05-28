package adapter

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Event 表示 OneBot V11 的事件结构
type Event struct {
	PostType    string      `json:"post_type"`    // 事件类型，如 message
	MessageType string      `json:"message_type"` // 消息类型，如 group
	MessageID   int64       `json:"message_id"`   // 消息ID
	GroupID     int64       `json:"group_id"`     // 群号
	UserID      int64       `json:"user_id"`      // 发送者QQ
	Message     string      `json:"message"`      // 原始消息内容
	SelfID      int64       `json:"self_id"`      // 机器人QQ
	Reply       ReplyAction `json:"reply"`        // 回复动作
}

// ReplyAction 表示回复动作结构
type ReplyAction struct {
	Async bool `json:"async"` // 是否异步发送
}

// Action 表示发送消息的动作
type Action struct {
	Action string                 `json:"action"` // 动作类型，如 send_group_msg
	Params map[string]interface{} `json:"params"` // 参数
}

// ParseEvent 将 WebSocket 收到的 byte 数组解析为标准的 Event 结构体
func ParseEvent(rawData []byte) (*Event, error) {
	var event Event

	// 尝试解析 JSON
	err := json.Unmarshal(rawData, &event)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %v", err)
	}

	// 忽略非消息事件或心跳包
	if event.PostType != "message" || event.MessageType != "group" {
		return nil, fmt.Errorf("忽略非群聊消息事件")
	}

	return &event, nil
}

// BuildSendGroupMsgAction 根据传入的群号和文本，构造回复内容的 Action JSON 格式
func BuildSendGroupMsgAction(groupID int64, text string) []byte {
	// 构造发送消息的动作
	action := Action{
		Action: "send_group_msg",
		Params: map[string]interface{}{
			"group_id": groupID,
			"message":  text,
		},
	}

	// 序列化为 JSON
	data, err := json.Marshal(action)
	if err != nil {
		log.Printf("构造发送消息动作失败: %v", err)
		return nil
	}

	return data
}

// ExtractCleanText 判断消息中是否包含 [CQ:at,qq={botQQ}]，剥离该 CQ 码并返回清理后的文本
func ExtractCleanText(rawMsg string, botQQ int64) (string, bool) {
	// 构造 @ 机器人的 CQ 码格式
	atCode := fmt.Sprintf("[CQ:at,qq=%d]", botQQ)

	// 检查是否包含 @ 机器人的 CQ 码
	isAt := strings.Contains(rawMsg, atCode)

	// 如果包含，则剥离该 CQ 码
	if isAt {
		cleanText := strings.ReplaceAll(rawMsg, atCode, "")
		cleanText = strings.TrimSpace(cleanText)
		return cleanText, true
	}

	// 不包含，返回原始文本
	return rawMsg, false
}
