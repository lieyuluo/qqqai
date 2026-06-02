package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
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

type groupFileURLRequest struct {
	GroupID int64  `json:"group_id"`
	FileID  string `json:"file_id"`
	BusID   int64  `json:"busid"`
}

type groupFileURLResponse struct {
	RetCode int `json:"retcode"`
	Data    struct {
		URL string `json:"url"`
	} `json:"data"`
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

func FetchGroupFileURL(ctx context.Context, httpBaseURL, accessToken string, groupID int64, fileID string, busID int64) (string, error) {
	payload := groupFileURLRequest{
		GroupID: groupID,
		FileID:  fileID,
		BusID:   busID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构造获取群文件链接请求失败: %w", err)
	}

	httpBaseURL = strings.TrimRight(strings.TrimSpace(httpBaseURL), "/")
	if httpBaseURL == "" {
		return "", fmt.Errorf("NapCat HTTP 地址不能为空")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpBaseURL+"/get_group_file_url", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建获取群文件链接请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	SetBearerAuthHeader(req, accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求群文件链接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("获取群文件链接接口返回状态码 %d", resp.StatusCode)
	}

	var result groupFileURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析群文件链接响应失败: %w", err)
	}
	if result.RetCode != 0 {
		return "", fmt.Errorf("获取群文件链接失败，retcode=%d", result.RetCode)
	}

	url := strings.TrimSpace(result.Data.URL)
	if url == "" {
		return "", fmt.Errorf("获取群文件链接响应缺少 data.url")
	}
	return url, nil
}

func SetBearerAuthHeader(req *http.Request, accessToken string) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
