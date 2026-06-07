package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"qqqai/config"

	"github.com/google/uuid"
)

type StoredFile struct {
	OriginalName string
	StoredName   string
	Path         string
	MimeType     string
	Size         int64
}

type Storage interface {
	Save(ctx context.Context, userID int64, file *multipart.FileHeader) (*StoredFile, error)
	Delete(ctx context.Context, path string) error
}

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) *LocalStorage {
	if strings.TrimSpace(root) == "" {
		root = config.GetUploadDir()
	}
	return &LocalStorage{root: root}
}

func (s *LocalStorage) Save(ctx context.Context, userID int64, file *multipart.FileHeader) (*StoredFile, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	originalName := filepath.Base(file.Filename)
	safeName := safeFileName(originalName)
	storedName := uuid.NewString() + "_" + safeName
	dir := filepath.Join(s.root, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, storedName)
	dst, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &StoredFile{
		OriginalName: originalName,
		StoredName:   storedName,
		Path:         path,
		MimeType:     file.Header.Get("Content-Type"),
		Size:         file.Size,
	}, nil
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var unsafeFileName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "upload"
	}
	name = unsafeFileName.ReplaceAllString(name, "_")
	if len(name) > 120 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		if len(base) > 100 {
			base = base[:100]
		}
		name = base + ext
	}
	return name
}
