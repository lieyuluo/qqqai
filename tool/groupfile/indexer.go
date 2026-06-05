package groupfile

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"qqqai/adapter"
	"qqqai/config"
	"qqqai/rag/rag_flow"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/document"
)

const legacyNapCatAccessToken = "yAEsax.~ofNwpFDk"

type IndexRequest struct {
	GroupID  int64  `json:"group_id"`
	FileID   string `json:"file_id"`
	BusID    int64  `json:"busid"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	FileURL  string `json:"file_url,omitempty"`
}

type IndexResponse struct {
	FileName string   `json:"file_name"`
	Count    int      `json:"count"`
	IDs      []string `json:"ids"`
	Message  string   `json:"message"`
}

func RequestFromEvent(event *adapter.Event) (IndexRequest, error) {
	if event == nil || event.File == nil {
		return IndexRequest{}, fmt.Errorf("群文件事件缺少文件信息")
	}

	fileName := strings.TrimSpace(event.File.Name)
	if fileName == "" {
		fileName = "未命名文件"
	}

	return IndexRequest{
		GroupID:  event.GroupID,
		FileID:   strings.TrimSpace(event.File.ID),
		BusID:    event.File.BusID,
		FileName: fileName,
		FileSize: event.File.Size,
		FileURL:  strings.TrimSpace(event.File.URL),
	}, nil
}

func ReplyForError(req IndexRequest, err error) string {
	fileName := req.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = "未命名文件"
	}
	return fmt.Sprintf("群文件 %s 索引失败: %v", fileName, err)
}

func Index(ctx context.Context, req IndexRequest) (*IndexResponse, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	fileURL := strings.TrimSpace(req.FileURL)
	if fileURL == "" {
		var err error
		fileURL, err = adapter.FetchGroupFileURL(ctx, config.GetNapCatHTTPBaseURL(), napCatAccessToken(), req.GroupID, req.FileID, req.BusID)
		if err != nil {
			return nil, fmt.Errorf("获取群文件 %s 下载地址失败: %w", req.FileName, err)
		}
	}

	path, err := download(ctx, req.FileName, fileURL, req.FileSize)
	if err != nil {
		return nil, fmt.Errorf("下载群文件 %s 失败: %w", req.FileName, err)
	}
	defer os.Remove(path)

	runnable, err := rag_flow.GetIndexingGraph()
	if err != nil {
		return nil, fmt.Errorf("索引图未初始化: %w", err)
	}

	ids, err := runnable.Invoke(ctx, document.Source{URI: path})
	if err != nil {
		return nil, fmt.Errorf("索引群文件 %s 失败: %w", req.FileName, err)
	}

	log.Printf("群文件 %s 索引完成，写入 %d 条记录", req.FileName, len(ids))
	message := fmt.Sprintf("群文件 %s 已完成索引，写入 %d 条记录。", req.FileName, len(ids))
	return &IndexResponse{
		FileName: req.FileName,
		Count:    len(ids),
		IDs:      ids,
		Message:  message,
	}, nil
}

func validateRequest(req IndexRequest) error {
	if strings.TrimSpace(req.FileName) == "" {
		return fmt.Errorf("文件名不能为空")
	}
	if strings.TrimSpace(req.FileURL) != "" {
		return nil
	}
	if req.GroupID == 0 {
		return fmt.Errorf("group_id 不能为空")
	}
	if strings.TrimSpace(req.FileID) == "" || req.BusID == 0 {
		return fmt.Errorf("事件中缺少文件标识，无法自动索引")
	}
	return nil
}

func download(ctx context.Context, fileName, fileURL string, fileSize int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	adapter.SetBearerAuthHeader(req, napCatAccessToken())

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

func napCatAccessToken() string {
	if token := config.GetNapCatAccessToken(); token != "" {
		return token
	}
	return legacyNapCatAccessToken
}
