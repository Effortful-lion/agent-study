// Package storage 提供 JSON 格式的持久化存储。
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
)

// DocumentStore 管理文档持久化。
type DocumentStore struct {
	path string
	mu   sync.RWMutex
}

// NewDocumentStore 创建文档存储。
func NewDocumentStore(path string) *DocumentStore {
	return &DocumentStore{path: path}
}

// Save 保存文档列表。
func (s *DocumentStore) Save(docs []DocumentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// Load 加载文档列表。
func (s *DocumentStore) Load() ([]DocumentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var docs []DocumentRecord
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ChunkStore 管理块持久化。
type ChunkStore struct {
	path string
	mu   sync.RWMutex
}

// NewChunkStore 创建块存储。
func NewChunkStore(path string) *ChunkStore {
	return &ChunkStore{path: path}
}

// Save 保存块列表。
func (s *ChunkStore) Save(chunks []ChunkRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// Load 加载块列表。
func (s *ChunkStore) Load() ([]ChunkRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var chunks []ChunkRecord
	if err := json.Unmarshal(data, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

// DocumentRecord 是文档的 JSON 持久化形式。
type DocumentRecord struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"`
	Title     string `json:"title"`
	Size      int64  `json:"size"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ChunkRecord 是块的 JSON 持久化形式。
type ChunkRecord struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	FilePath   string `json:"file_path"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	StartPos   int    `json:"start_pos"`
	EndPos     int    `json:"end_pos"`
	Keyword    string `json:"keyword,omitempty"`
}

// ToDocumentRecord 转换为持久化记录。
func ToDocumentRecord(d *document.Document) DocumentRecord {
	return DocumentRecord{
		ID:        d.ID,
		FilePath:  d.FilePath,
		Title:     d.Title,
		Size:      d.Size,
		Hash:      d.Hash,
		CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToChunkRecord 转换为持久化记录。
func ToChunkRecord(c *document.Chunk) ChunkRecord {
	return ChunkRecord{
		ID:         c.ID,
		DocumentID: c.DocumentID,
		FilePath:   c.FilePath,
		Title:      c.Title,
		Content:    c.Content,
		StartPos:   c.StartPos,
		EndPos:     c.EndPos,
		Keyword:    c.Keyword,
	}
}
