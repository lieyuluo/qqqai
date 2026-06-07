package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"qqqai/config"
	"qqqai/internal/entity"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func Init(ctx context.Context) error {
	if config.GlobalConfig == nil {
		return fmt.Errorf("config not initialized")
	}
	c := config.GlobalConfig.MySQLConf
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database)

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(20)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(30 * time.Minute)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return err
	}
	db = conn
	return nil
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func DB() (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return db, nil
}

func CreateUser(ctx context.Context, email, username, passwordHash, role string) (*entity.User, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	if role == "" {
		role = entity.RoleUser
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, status)
		VALUES (?, ?, ?, ?, ?)`,
		email, username, passwordHash, role, entity.UserStatusActive)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetUserByID(ctx, id)
}

func UpsertAdmin(ctx context.Context, email, username, passwordHash string) error {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(passwordHash) == "" {
		return nil
	}
	conn, err := DB()
	if err != nil {
		return err
	}
	if strings.TrimSpace(username) == "" {
		username = "Admin"
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, status)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			username = VALUES(username),
			password_hash = VALUES(password_hash),
			role = VALUES(role),
			status = VALUES(status),
			updated_at = CURRENT_TIMESTAMP`,
		strings.ToLower(strings.TrimSpace(email)), username, passwordHash, entity.RoleAdmin, entity.UserStatusActive)
	return err
}

func GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, email, username, password_hash, role, status, created_at, updated_at
		FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

func GetUserByID(ctx context.Context, id int64) (*entity.User, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, email, username, password_hash, role, status, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func ListUsers(ctx context.Context, limit, offset int) ([]entity.User, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT id, email, username, password_hash, role, status, created_at, updated_at
		FROM users ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *user)
	}
	return out, rows.Err()
}

func CreateConversation(ctx context.Context, userID int64, title string) (*entity.Conversation, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New conversation"
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO conversations (user_id, title) VALUES (?, ?)`, userID, title)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetConversationForUser(ctx, userID, id)
}

func GetConversationForUser(ctx context.Context, userID, id int64) (*entity.Conversation, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, user_id, title, created_at, updated_at, deleted_at
		FROM conversations WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, id, userID)
	return scanConversation(row)
}

func ListConversations(ctx context.Context, userID int64, limit, offset int) ([]entity.Conversation, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, title, created_at, updated_at, deleted_at
		FROM conversations
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConversationList(rows)
}

func ListAllConversations(ctx context.Context, limit, offset int) ([]entity.Conversation, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, title, created_at, updated_at, deleted_at
		FROM conversations
		ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConversationList(rows)
}

func DeleteConversation(ctx context.Context, userID, id int64) error {
	conn, err := DB()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `UPDATE conversations SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func CreateMessage(ctx context.Context, conversationID, userID int64, role, content string) (*entity.Message, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO messages (conversation_id, user_id, role, content)
		VALUES (?, ?, ?, ?)`, conversationID, userID, role, content)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, conversation_id, user_id, role, content, created_at
		FROM messages WHERE id = ?`, id)
	return scanMessage(row)
}

func ListMessages(ctx context.Context, conversationID, userID int64, limit, offset int) ([]entity.Message, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.user_id, m.role, m.content, m.created_at
		FROM messages m
		INNER JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = ? AND c.user_id = ? AND c.deleted_at IS NULL
		ORDER BY m.id ASC LIMIT ? OFFSET ?`, conversationID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.Message
	for rows.Next() {
		msg, err := scanMessageRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *msg)
	}
	return out, rows.Err()
}

func TouchConversation(ctx context.Context, id int64) error {
	conn, err := DB()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func CreateFile(ctx context.Context, file *entity.FileRecord) (*entity.FileRecord, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO files (user_id, original_name, stored_name, path, mime_type, size, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		file.UserID, file.OriginalName, file.StoredName, file.Path, file.MimeType, file.Size, file.Status)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetFileByID(ctx, id)
}

func GetFileForUser(ctx context.Context, userID, id int64) (*entity.FileRecord, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, user_id, original_name, stored_name, path, mime_type, size, status, error, indexed_count, created_at, updated_at
		FROM files WHERE id = ? AND user_id = ?`, id, userID)
	return scanFile(row)
}

func GetFileByID(ctx context.Context, id int64) (*entity.FileRecord, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, user_id, original_name, stored_name, path, mime_type, size, status, error, indexed_count, created_at, updated_at
		FROM files WHERE id = ?`, id)
	return scanFile(row)
}

func ListFiles(ctx context.Context, userID int64, limit, offset int) ([]entity.FileRecord, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, original_name, stored_name, path, mime_type, size, status, error, indexed_count, created_at, updated_at
		FROM files WHERE user_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFileList(rows)
}

func ListAllFiles(ctx context.Context, limit, offset int) ([]entity.FileRecord, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	limit, offset = normalizePage(limit, offset)
	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, original_name, stored_name, path, mime_type, size, status, error, indexed_count, created_at, updated_at
		FROM files ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFileList(rows)
}

func DeleteFile(ctx context.Context, userID, id int64) error {
	conn, err := DB()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `DELETE FROM files WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func UpdateFileStatus(ctx context.Context, id int64, status, errorText string, indexedCount int) error {
	conn, err := DB()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `
		UPDATE files SET status = ?, error = ?, indexed_count = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, errorText, indexedCount, id)
	return err
}

func CreateModelCallLog(ctx context.Context, log entity.ModelCallLog) (int64, error) {
	conn, err := DB()
	if err != nil {
		return 0, err
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO model_call_logs (user_id, conversation_id, request_type, prompt, response, latency_ms, status, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.UserID, log.ConversationID, log.RequestType, log.Prompt, log.Response, log.LatencyMS, log.Status, log.Error)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func AdminStats(ctx context.Context) (*entity.AdminStats, error) {
	conn, err := DB()
	if err != nil {
		return nil, err
	}
	stats := &entity.AdminStats{}
	queries := []struct {
		sql string
		dst *int64
	}{
		{`SELECT COUNT(*) FROM users`, &stats.Users},
		{`SELECT COUNT(*) FROM conversations WHERE deleted_at IS NULL`, &stats.Conversations},
		{`SELECT COUNT(*) FROM messages`, &stats.Messages},
		{`SELECT COUNT(*) FROM files`, &stats.Files},
		{`SELECT COUNT(*) FROM model_call_logs`, &stats.ModelCalls},
		{`SELECT COUNT(*) FROM files WHERE status = 'indexed'`, &stats.IndexedFiles},
		{`SELECT COUNT(*) FROM files WHERE status = 'failed'`, &stats.FailedFiles},
	}
	for _, q := range queries {
		if err := conn.QueryRowContext(ctx, q.sql).Scan(q.dst); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*entity.User, error) {
	user := &entity.User{}
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func scanUserRows(rows *sql.Rows) (*entity.User, error) {
	return scanUser(rows)
}

func scanConversation(row rowScanner) (*entity.Conversation, error) {
	conversation := &entity.Conversation{}
	var deletedAt sql.NullTime
	err := row.Scan(&conversation.ID, &conversation.UserID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		conversation.DeletedAt = &deletedAt.Time
	}
	return conversation, nil
}

func scanConversationList(rows *sql.Rows) ([]entity.Conversation, error) {
	var out []entity.Conversation
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *conversation)
	}
	return out, rows.Err()
}

func scanMessage(row rowScanner) (*entity.Message, error) {
	msg := &entity.Message{}
	err := row.Scan(&msg.ID, &msg.ConversationID, &msg.UserID, &msg.Role, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func scanMessageRows(rows *sql.Rows) (*entity.Message, error) {
	return scanMessage(rows)
}

func scanFile(row rowScanner) (*entity.FileRecord, error) {
	file := &entity.FileRecord{}
	var errorText sql.NullString
	err := row.Scan(
		&file.ID, &file.UserID, &file.OriginalName, &file.StoredName, &file.Path,
		&file.MimeType, &file.Size, &file.Status, &errorText, &file.IndexedCount,
		&file.CreatedAt, &file.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if errorText.Valid {
		file.Error = errorText.String
	}
	return file, nil
}

func scanFileList(rows *sql.Rows) ([]entity.FileRecord, error) {
	var out []entity.FileRecord
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *file)
	}
	return out, rows.Err()
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
