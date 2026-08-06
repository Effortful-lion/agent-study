// Package config 管理 mini-kb 的全局配置。
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config 是 mini-kb 的全局配置。
type Config struct {
	// StorageDir 是 JSON 存储目录（documents.json, chunks.json, sessions/）。
	StorageDir string
	// ChunkSize 是文本块的最大字符数（UTF-8 码元）。
	ChunkSize int
	// ChunkOverlap 是相邻块之间的重叠字符数。
	ChunkOverlap int
	// TopK 是检索时返回的最大结果数。
	TopK int
	// Provider 是 LLM 提供者名称（openai, deepseek, doubao, qwen 等）。
	Provider string
	// Model 是模型名称，空时使用提供者默认值。
	Model string
	// APIKey 是 API 密钥，优先从环境变量或 .env 读取。
	APIKey string
	// BaseURL 是 API 端点，空时使用提供者默认值。
	BaseURL string
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		StorageDir:   filepath.Join(home, ".mini-kb"),
		ChunkSize:    800,
		ChunkOverlap: 150,
		TopK:         5,
	}
}

// Validate 检查配置是否可用。
func (c *Config) Validate() error {
	if c.StorageDir == "" {
		return fmt.Errorf("storage dir 不能为空")
	}
	if c.ChunkSize <= 0 {
		return fmt.Errorf("chunk size 必须大于 0")
	}
	if c.ChunkOverlap < 0 {
		return fmt.Errorf("chunk overlap 不能为负数")
	}
	if c.ChunkOverlap >= c.ChunkSize {
		return fmt.Errorf("chunk overlap (%d) 必须小于 chunk size (%d)", c.ChunkOverlap, c.ChunkSize)
	}
	if c.TopK <= 0 {
		return fmt.Errorf("topK 必须大于 0")
	}
	return nil
}
