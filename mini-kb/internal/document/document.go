// Package document 定义文档和文本块的领域模型。
package document

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Document 表示一个导入的源文档。
type Document struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Title     string    `json:"title"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Chunk 表示文档的一个文本块。
type Chunk struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	FilePath   string `json:"file_path"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	StartPos   int    `json:"start_pos"`
	EndPos     int    `json:"end_pos"`
	Keyword    string `json:"keyword,omitempty"`
}

// NewDocument 从文件路径创建一个 Document。
func NewDocument(filePath string) (*Document, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("hash file failed: %w", err)
	}

	title := extractTitle(filePath, info.Name())
	return &Document{
		ID:        hashString(filePath + info.ModTime().String()),
		FilePath:  filePath,
		Title:     title,
		Size:      info.Size(),
		Hash:      fmt.Sprintf("%x", h.Sum(nil)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// extractTitle 从文件名或 Markdown 标题提取文档标题。
func extractTitle(filePath, fileName string) string {
	// 优先从文件内容提取第一个 # 标题
	if strings.HasSuffix(strings.ToLower(fileName), ".md") {
		if content, err := os.ReadFile(filePath); err == nil {
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "# ") {
					return strings.TrimSpace(line[2:])
				}
				if line == "" {
					continue
				}
				break
			}
		}
	}
	base := fileName
	if ext := filepath.Ext(fileName); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// hashString 计算字符串的 MD5。
func hashString(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h[:])
}
