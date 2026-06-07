package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"qqqai/internal/dao"
	"qqqai/internal/entity"
	"qqqai/internal/middleware"
	"qqqai/internal/response"
	"qqqai/internal/service"
	localstorage "qqqai/internal/storage"
	"qqqai/internal/worker"

	"github.com/gogf/gf/v2/net/ghttp"
)

type App struct {
	Auth       *service.AuthService
	Chat       *service.ChatService
	SQL        *service.SQLService
	ChatPool   *worker.ChatTaskPool
	FileWorker *worker.FileIndexWorker
	Storage    localstorage.Storage
}

func NewApp(auth *service.AuthService, chat *service.ChatService, sqlService *service.SQLService, chatPool *worker.ChatTaskPool, fileWorker *worker.FileIndexWorker, storage localstorage.Storage) *App {
	return &App{
		Auth:       auth,
		Chat:       chat,
		SQL:        sqlService,
		ChatPool:   chatPool,
		FileWorker: fileWorker,
		Storage:    storage,
	}
}

func RegisterRoutes(s *ghttp.Server, app *App, wsHandler func(*ghttp.Request)) {
	s.Use(middleware.RequestID, middleware.Recovery, middleware.CORS)
	s.BindHandler("GET:/ws", wsHandler)
	s.BindHandler("POST:/api/auth/register", app.Register)
	s.BindHandler("POST:/api/auth/login", app.Login)

	s.Group("/api", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Auth)
		group.GET("/auth/me", app.Me)
		group.POST("/conversations", app.CreateConversation)
		group.GET("/conversations", app.ListConversations)
		group.GET("/conversations/{id}/messages", app.ListMessages)
		group.DELETE("/conversations/{id}", app.DeleteConversation)
		group.POST("/chat", app.ChatOnce)
		group.POST("/chat/stream", app.ChatStream)
		group.POST("/files/upload", app.UploadFile)
		group.GET("/files", app.ListFiles)
		group.DELETE("/files/{id}", app.DeleteFile)
		group.POST("/sql/generate", app.GenerateSQL)
		group.POST("/sql/execute", app.ExecuteSQL)

		group.Group("/admin", func(admin *ghttp.RouterGroup) {
			admin.Middleware(middleware.Admin)
			admin.GET("/stats", app.AdminStats)
			admin.GET("/users", app.AdminUsers)
			admin.GET("/conversations", app.AdminConversations)
			admin.GET("/files", app.AdminFiles)
		})
	})
}

func (a *App) Register(r *ghttp.Request) {
	var input service.RegisterInput
	if !decodeJSON(r, &input) {
		return
	}
	user, err := a.Auth.Register(r.Context(), input)
	if err != nil {
		response.BadRequest(r, err.Error())
		return
	}
	response.Created(r, user)
}

func (a *App) Login(r *ghttp.Request) {
	var input service.LoginInput
	if !decodeJSON(r, &input) {
		return
	}
	user, token, err := a.Auth.Login(r.Context(), input)
	if err != nil {
		response.Unauthorized(r, err.Error())
		return
	}
	response.Success(r, map[string]any{"token": token, "user": user})
}

func (a *App) Me(r *ghttp.Request) {
	response.Success(r, middleware.CurrentUser(r))
}

func (a *App) CreateConversation(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	var input struct {
		Title string `json:"title"`
	}
	if !decodeJSON(r, &input) {
		return
	}
	conversation, err := dao.CreateConversation(r.Context(), user.ID, input.Title)
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Created(r, conversation)
}

func (a *App) ListConversations(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	conversations, err := dao.ListConversations(r.Context(), user.ID, queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, conversations)
}

func (a *App) ListMessages(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	conversationID, ok := routeID(r)
	if !ok {
		return
	}
	messages, err := dao.ListMessages(r.Context(), conversationID, user.ID, queryInt(r, "limit", 100), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, messages)
}

func (a *App) DeleteConversation(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	conversationID, ok := routeID(r)
	if !ok {
		return
	}
	if err := dao.DeleteConversation(r.Context(), user.ID, conversationID); err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, map[string]bool{"deleted": true})
}

func (a *App) ChatOnce(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	var input struct {
		ConversationID int64  `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if !decodeJSON(r, &input) {
		return
	}
	conversation, ok := a.ensureConversation(r, user.ID, input.ConversationID, input.Message)
	if !ok {
		return
	}
	if _, err := dao.CreateMessage(r.Context(), conversation.ID, user.ID, entity.MessageRoleUser, input.Message); err != nil {
		response.Internal(r, err.Error())
		return
	}
	resp, err := a.ChatPool.Submit(r.Context(), service.ChatRequest{
		SessionID: webSessionID(user.ID, conversation.ID),
		Query:     input.Message,
	})
	status := "success"
	errorText := ""
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}
	logID, _ := dao.CreateModelCallLog(r.Context(), entity.ModelCallLog{
		UserID:         user.ID,
		ConversationID: conversation.ID,
		RequestType:    "chat",
		Prompt:         input.Message,
		Response:       resp.Reply,
		LatencyMS:      resp.LatencyMS,
		Status:         status,
		Error:          errorText,
	})
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	assistantMessage, err := dao.CreateMessage(r.Context(), conversation.ID, user.ID, entity.MessageRoleAssistant, resp.Reply)
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	_ = dao.TouchConversation(r.Context(), conversation.ID)
	response.Success(r, map[string]any{
		"conversation": conversation,
		"message":      assistantMessage,
		"reply":        resp.Reply,
		"log_id":       logID,
	})
}

func (a *App) ChatStream(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	var input struct {
		ConversationID int64  `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if !decodeJSON(r, &input) {
		return
	}
	conversation, ok := a.ensureConversation(r, user.ID, input.ConversationID, input.Message)
	if !ok {
		return
	}
	if _, err := dao.CreateMessage(r.Context(), conversation.ID, user.ID, entity.MessageRoleUser, input.Message); err != nil {
		response.Internal(r, err.Error())
		return
	}
	chunks, err := a.Chat.Stream(r.Context(), service.ChatRequest{
		SessionID: webSessionID(user.ID, conversation.ID),
		Query:     input.Message,
	})
	if err != nil {
		response.Internal(r, err.Error())
		return
	}

	r.Response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.WriteStatus(http.StatusOK)
	r.Response.Flush()

	var reply strings.Builder
	started := time.Now()
	for {
		select {
		case <-r.Context().Done():
			return
		case chunk, ok := <-chunks:
			if !ok {
				return
			}
			if chunk.Content != "" {
				reply.WriteString(chunk.Content)
			}
			writeSSE(r, chunk)
			if chunk.Done {
				if chunk.Error == "" {
					text := reply.String()
					_, _ = dao.CreateMessage(r.Context(), conversation.ID, user.ID, entity.MessageRoleAssistant, text)
					_, _ = dao.CreateModelCallLog(r.Context(), entity.ModelCallLog{
						UserID:         user.ID,
						ConversationID: conversation.ID,
						RequestType:    "chat_stream",
						Prompt:         input.Message,
						Response:       text,
						LatencyMS:      time.Since(started).Milliseconds(),
						Status:         "success",
					})
					_ = dao.TouchConversation(r.Context(), conversation.ID)
				}
				return
			}
		}
	}
}

func (a *App) UploadFile(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	if err := r.Request.ParseMultipartForm(64 << 20); err != nil {
		response.BadRequest(r, "invalid multipart upload")
		return
	}
	files := r.Request.MultipartForm.File["file"]
	if len(files) == 0 {
		response.BadRequest(r, "file is required")
		return
	}
	stored, err := a.Storage.Save(r.Context(), user.ID, files[0])
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	record, err := dao.CreateFile(r.Context(), &entity.FileRecord{
		UserID:       user.ID,
		OriginalName: stored.OriginalName,
		StoredName:   stored.StoredName,
		Path:         stored.Path,
		MimeType:     stored.MimeType,
		Size:         stored.Size,
		Status:       entity.FileStatusPending,
	})
	if err != nil {
		_ = a.Storage.Delete(r.Context(), stored.Path)
		response.Internal(r, err.Error())
		return
	}
	if err := a.FileWorker.Submit(worker.FileIndexTask{FileID: record.ID, Path: record.Path}); err != nil {
		record.Error = "saved, but index queue failed: " + err.Error()
		response.JSON(r, http.StatusAccepted, response.CodeOK, record.Error, record)
		return
	}
	response.Created(r, record)
}

func (a *App) ListFiles(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	files, err := dao.ListFiles(r.Context(), user.ID, queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, files)
}

func (a *App) DeleteFile(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	fileID, ok := routeID(r)
	if !ok {
		return
	}
	file, err := dao.GetFileForUser(r.Context(), user.ID, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(r, "file not found")
			return
		}
		response.Internal(r, err.Error())
		return
	}
	if err := dao.DeleteFile(r.Context(), user.ID, fileID); err != nil {
		response.Internal(r, err.Error())
		return
	}
	if err := a.Storage.Delete(r.Context(), file.Path); err != nil {
		log.Printf("delete local file failed: %v", err)
	}
	response.Success(r, map[string]bool{"deleted": true})
}

func (a *App) GenerateSQL(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	var input struct {
		Prompt string `json:"prompt"`
	}
	if !decodeJSON(r, &input) {
		return
	}
	started := time.Now()
	sqlText, err := a.SQL.Generate(r.Context(), input.Prompt)
	status, errorText := "success", ""
	if err != nil {
		status, errorText = "failed", err.Error()
	}
	logID, _ := dao.CreateModelCallLog(r.Context(), entity.ModelCallLog{
		UserID:      user.ID,
		RequestType: "sql_generate",
		Prompt:      input.Prompt,
		Response:    sqlText,
		LatencyMS:   time.Since(started).Milliseconds(),
		Status:      status,
		Error:       errorText,
	})
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, map[string]any{"sql": sqlText, "log_id": logID})
}

func (a *App) ExecuteSQL(r *ghttp.Request) {
	user := middleware.CurrentUser(r)
	var input struct {
		SQL     string `json:"sql"`
		Confirm bool   `json:"confirm"`
	}
	if !decodeJSON(r, &input) {
		return
	}
	started := time.Now()
	executedSQL, result, err := a.SQL.Execute(r.Context(), input.SQL, input.Confirm)
	status, errorText := "success", ""
	if err != nil {
		status, errorText = "failed", err.Error()
	}
	logID, _ := dao.CreateModelCallLog(r.Context(), entity.ModelCallLog{
		UserID:      user.ID,
		RequestType: "sql_execute",
		Prompt:      input.SQL,
		Response:    result,
		LatencyMS:   time.Since(started).Milliseconds(),
		Status:      status,
		Error:       errorText,
	})
	if err != nil {
		response.BadRequest(r, err.Error())
		return
	}
	response.Success(r, map[string]any{"sql": executedSQL, "result": result, "log_id": logID})
}

func (a *App) AdminStats(r *ghttp.Request) {
	stats, err := dao.AdminStats(r.Context())
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, stats)
}

func (a *App) AdminUsers(r *ghttp.Request) {
	users, err := dao.ListUsers(r.Context(), queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, users)
}

func (a *App) AdminConversations(r *ghttp.Request) {
	conversations, err := dao.ListAllConversations(r.Context(), queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, conversations)
}

func (a *App) AdminFiles(r *ghttp.Request) {
	files, err := dao.ListAllFiles(r.Context(), queryInt(r, "limit", 20), queryInt(r, "offset", 0))
	if err != nil {
		response.Internal(r, err.Error())
		return
	}
	response.Success(r, files)
}

func (a *App) ensureConversation(r *ghttp.Request, userID, conversationID int64, message string) (*entity.Conversation, bool) {
	if strings.TrimSpace(message) == "" {
		response.BadRequest(r, "message is required")
		return nil, false
	}
	if conversationID > 0 {
		conversation, err := dao.GetConversationForUser(r.Context(), userID, conversationID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.NotFound(r, "conversation not found")
				return nil, false
			}
			response.Internal(r, err.Error())
			return nil, false
		}
		return conversation, true
	}
	title := strings.TrimSpace(message)
	if len([]rune(title)) > 24 {
		title = string([]rune(title)[:24])
	}
	conversation, err := dao.CreateConversation(r.Context(), userID, title)
	if err != nil {
		response.Internal(r, err.Error())
		return nil, false
	}
	return conversation, true
}

func decodeJSON(r *ghttp.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		response.BadRequest(r, "invalid json body")
		return false
	}
	return true
}

func routeID(r *ghttp.Request) (int64, bool) {
	value := r.GetRouter("id").String()
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(r, "invalid id")
		return 0, false
	}
	return id, true
}

func queryInt(r *ghttp.Request, name string, def int) int {
	raw := strings.TrimSpace(r.GetQuery(name).String())
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return value
}

func writeSSE(r *ghttp.Request, chunk service.ChatChunk) {
	data, _ := json.Marshal(chunk)
	r.Response.Write([]byte("data: "))
	r.Response.Write(data)
	r.Response.Write([]byte("\n\n"))
	r.Response.Flush()
}

func webSessionID(userID, conversationID int64) string {
	return fmt.Sprintf("web:%d:%d", userID, conversationID)
}

func EnsureUploadDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.MkdirAll(path, 0755)
}
