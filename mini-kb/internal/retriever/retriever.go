// Package retriever 实现基于关键词的混合检索。
package retriever

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
)

// Result 是一次检索的结果。
type Result struct {
	Chunk    *document.Chunk `json:"chunk"`
	Score    float64         `json:"score"`
	Title    string          `json:"title"`
	FilePath string          `json:"file_path"`
	Content  string          `json:"content"`
}

// Retriever 执行关键词混合检索。
type Retriever struct {
	chunks []*document.Chunk
}

// NewRetriever 创建一个新的检索器。
func NewRetriever() *Retriever {
	return &Retriever{}
}

// LoadChunks 加载所有块到内存中。
func (r *Retriever) LoadChunks(chunks []*document.Chunk) {
	r.chunks = append(r.chunks, chunks...)
}

// Search 执行混合检索并返回 TopK 结果。
func (r *Retriever) Search(query string, topK int) ([]Result, error) {
	if len(r.chunks) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	scored := make(map[string]*Result)
	for _, chunk := range r.chunks {
		score := r.score(chunk, queryTerms)
		if score <= 0 {
			continue
		}
		res := &Result{
			Chunk:    chunk,
			Score:    score,
			Title:    chunk.Title,
			FilePath: chunk.FilePath,
			Content:  chunk.Content,
		}
		scored[chunk.ID] = res
	}

	// 按分数排序并取 TopK
	var results []Result
	for _, r := range scored {
		results = append(results, *r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// score 计算块与查询的相关性分数。
// 算法：标题命中加权 5，内容命中加权 1，关键词命中加权 2。
func (r *Retriever) Score(chunk *document.Chunk, queryTerms []string) float64 {
	return r.score(chunk, queryTerms)
}

func (r *Retriever) score(chunk *document.Chunk, queryTerms []string) float64 {
	if chunk == nil {
		return 0
	}

	var score float64
	titleLower := strings.ToLower(chunk.Title)
	contentLower := strings.ToLower(chunk.Content)
	keywordLower := strings.ToLower(chunk.Keyword)

	for _, term := range queryTerms {
		t := strings.ToLower(term)
		// 标题精确匹配
		if strings.Contains(titleLower, t) {
			score += 5.0
		}
		// 关键词字段
		if strings.Contains(keywordLower, t) {
			score += 2.0
		}
		// 内容匹配
		count := strings.Count(contentLower, t)
		score += float64(count) * 1.0
	}

	// 长度归一化（更长的块可能包含更多信息，但不应该过度加分）
	length := len(chunk.Content)
	if length > 0 {
		score /= (1.0 + float64(length)/1000.0)
	}

	return score
}

// tokenize 对文本做简单分词。
func tokenize(text string) []string {
	text = strings.ToLower(text)
	replacer := strings.NewReplacer(
		",", " ", ".", " ", ";", " ", ":", " ", "!", " ", "?", " ",
		"，", " ", "。", " ", "；", " ", "：", " ", "！", " ", "？", " ",
		"\n", " ", "\t", " ", "\r", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ", "\"", " ", "'", " ",
	)
	text = replacer.Replace(text)

	var stopwords = map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "to": true, "of": true,
		"in": true, "for": true, "on": true, "with": true, "at": true,
		"by": true, "from": true, "and": true, "or": true, "if": true,
		"this": true, "that": true, "it": true, "as": true, "not": true,
		"but": true, "so": true, "no": true, "yes": true,
		"的": true, "了": true, "在": true, "是": true, "和": true,
		"就": true, "不": true, "也": true, "都": true, "要": true,
		"去": true, "会": true, "着": true, "看": true, "好": true,
	}

	fields := strings.Fields(text)
	var words []string
	for _, f := range fields {
		if len(f) < 2 || stopwords[f] {
			continue
		}
		words = append(words, f)
	}

	// 中文 2-gram
	var result []string
	for _, w := range words {
		if isChineseWord(w) {
			for i := 0; i < len(w)-1; i++ {
				result = append(result, w[i:i+2])
			}
		} else {
			result = append(result, w)
		}
	}

	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, w := range result {
		if !seen[w] {
			seen[w] = true
			unique = append(unique, w)
		}
	}
	return unique
}

func isChineseWord(s string) bool {
	for _, r := range s {
		if r >= '一' && r <= '鿿' {
			return true
		}
	}
	return false
}

// SearchByTitle 按标题做前缀/包含匹配。
func SearchByTitle(chunks []*document.Chunk, query string) []document.Chunk {
	var results []document.Chunk
	q := strings.ToLower(strings.TrimSpace(query))
	for _, c := range chunks {
		t := strings.ToLower(c.Title)
		if strings.Contains(t, q) {
			results = append(results, *c)
		}
	}
	return results
}

// FormatResult 将检索结果格式化为易读文本。
func FormatResult(r Result) string {
	return fmt.Sprintf("[%s] (%.1f) %s\n%s",
		r.FilePath, r.Score, r.Title, truncate(r.Content, 300))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
