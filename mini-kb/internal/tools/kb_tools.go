// Package tools 提供知识库检索工具，供 Agent 调用。
package tools

import (
	"context"
	"fmt"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/retriever"
)

// KBRetriever 封装知识库检索能力。
type KBRetriever struct {
	chunks []*document.Chunk
	docDir string
	topK   int
}

// NewKBRetriever 创建知识库检索器。
func NewKBRetriever(docDir string, topK int) *KBRetriever {
	return &KBRetriever{docDir: docDir, topK: topK}
}

// LoadChunks 加载所有块。
func (k *KBRetriever) LoadChunks(chunks []*document.Chunk) {
	k.chunks = append(k.chunks, chunks...)
}

// ChunkCount 返回当前块数量。
func (k *KBRetriever) ChunkCount() int {
	return len(k.chunks)
}

// Chunks 返回所有块（只读）。
func (k *KBRetriever) Chunks() []*document.Chunk {
	return k.chunks
}

// SearchKnowledge 搜索知识库，返回 TopK 结果。
func (k *KBRetriever) SearchKnowledge(ctx context.Context, args map[string]any) (any, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("缺少 query 参数")
	}

	topK := k.topK
	if v, ok := args["top_k"].(float64); ok && v > 0 {
		topK = int(v)
	}

	r := retriever.NewRetriever()
	r.LoadChunks(k.chunks)
	results, err := r.Search(query, topK)
	if err != nil {
		return nil, fmt.Errorf("检索失败: %w", err)
	}
	if len(results) == 0 {
		return "未找到相关内容", nil
	}

	var parts []string
	for i, res := range results {
		parts = append(parts, fmt.Sprintf("=== 结果 %d ===\n来源: %s\n标题: %s\n分数: %.1f\n内容:\n%s\n",
			i+1, res.FilePath, res.Title, res.Score, res.Content))
	}
	return fmt.Sprintf("找到 %d 条结果：\n%s", len(results), join(parts)), nil
}

// ReadDocument 读取指定文档的完整内容。
func (k *KBRetriever) ReadDocument(ctx context.Context, args map[string]any) (any, error) {
	docID, ok := args["doc_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("缺少 doc_id 参数")
	}

	// 搜索匹配的块
	var matched []*document.Chunk
	for _, c := range k.chunks {
		if c.DocumentID == docID {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("未找到文档 %s", docID)
	}

	var b []byte
	b = append(b, fmt.Sprintf("文档: %s\n", matched[0].Title)...)
	b = append(b, fmt.Sprintf("文件: %s\n", matched[0].FilePath)...)
	b = append(b, "--- 完整内容 ---\n"...)
	for _, c := range matched {
		b = append(b, c.Content...)
		b = append(b, '\n')
	}
	return string(b), nil
}

// GetChunk 获取指定块的完整内容。
func (k *KBRetriever) GetChunk(ctx context.Context, args map[string]any) (any, error) {
	chunkID, ok := args["chunk_id"].(string)
	if !ok || chunkID == "" {
		return nil, fmt.Errorf("缺少 chunk_id 参数")
	}

	for _, c := range k.chunks {
		if c.ID == chunkID {
			return fmt.Sprintf("块 %s\n来源: %s\n标题: %s\n位置: %d-%d\n内容:\n%s",
				c.ID, c.FilePath, c.Title, c.StartPos, c.EndPos, c.Content), nil
		}
	}
	return nil, fmt.Errorf("未找到块 %s", chunkID)
}

// ListDocuments 列出所有已索引的文档。
func (k *KBRetriever) ListDocuments(ctx context.Context, args map[string]any) (any, error) {
	seen := make(map[string]bool)
	var docs []string
	for _, c := range k.chunks {
		key := c.DocumentID + "|" + c.FilePath + "|" + c.Title
		if seen[key] {
			continue
		}
		seen[key] = true
		docs = append(docs, fmt.Sprintf("- %s (%s)", c.Title, c.FilePath))
	}
	if len(docs) == 0 {
		return "知识库为空", nil
	}
	return fmt.Sprintf("已索引 %d 个文档：\n%s", len(docs), join(docs)), nil
}

func join(parts []string) string {
	var b []byte
	for i, p := range parts {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, p...)
	}
	return string(b)
}
