package entity

import "time"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	UserStatusActive = "active"

	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"

	FileStatusPending  = "pending"
	FileStatusIndexing = "indexing"
	FileStatusIndexed  = "indexed"
	FileStatusFailed   = "failed"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Conversation struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	UserID         int64     `json:"user_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type FileRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	OriginalName string    `json:"original_name"`
	StoredName   string    `json:"stored_name"`
	Path         string    `json:"path"`
	MimeType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	IndexedCount int       `json:"indexed_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ModelCallLog struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	ConversationID int64     `json:"conversation_id"`
	RequestType    string    `json:"request_type"`
	Prompt         string    `json:"prompt"`
	Response       string    `json:"response"`
	LatencyMS      int64     `json:"latency_ms"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type AdminStats struct {
	Users         int64 `json:"users"`
	Conversations int64 `json:"conversations"`
	Messages      int64 `json:"messages"`
	Files         int64 `json:"files"`
	ModelCalls    int64 `json:"model_calls"`
	IndexedFiles  int64 `json:"indexed_files"`
	FailedFiles   int64 `json:"failed_files"`
}
