// Package index 负责将文档切分成文本块（chunk）并构建索引。
package index

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
)

// Splitter 将文档内容切分成文本块。
type Splitter struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewSplitter 创建一个新的 Splitter。
func NewSplitter(chunkSize, chunkOverlap int) *Splitter {
	return &Splitter{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap}
}

// Chunk 将文档内容切分成多个文本块。
func (s *Splitter) Chunk(doc *document.Document, content string) ([]document.Chunk, error) {
	if doc == nil {
		return nil, errors.New("document 为空")
	}
	if content == "" {
		return nil, nil
	}

	size := utf8.RuneCountInString(content)
	if size <= s.ChunkSize {
		chunkID := fmt.Sprintf("%s-0", doc.ID)
		return []document.Chunk{
			{
				ID:         chunkID,
				DocumentID: doc.ID,
				FilePath:   doc.FilePath,
				Title:      doc.Title,
				Content:    content,
				StartPos:   0,
				EndPos:     size,
			},
		}, nil
	}

	var chunks []document.Chunk
	runes := []rune(content)
	pos := 0
	chunkIdx := 0

	for pos < len(runes) {
		end := pos + s.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// 尝试在换行符处断开
		text := string(runes[pos:end])
		if breakPos := findBreakPoint(text, s.ChunkSize); breakPos > 0 && breakPos < len(text) {
			end = pos + utf8.RuneCountInString(text[:breakPos])
		}

		chunkText := string(runes[pos:end])
		chunkID := fmt.Sprintf("%s-%d", doc.ID, chunkIdx)

		chunks = append(chunks, document.Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			FilePath:   doc.FilePath,
			Title:      doc.Title,
			Content:    chunkText,
			StartPos:   pos,
			EndPos:     end,
		})

		nextPos := end - s.ChunkOverlap
		if nextPos <= pos {
			nextPos = pos + s.ChunkSize/2
		}
		pos = nextPos
		chunkIdx++
	}

	return chunks, nil
}

// findBreakPoint 在文本中找到合适的断开位置（优先换行符）。
func findBreakPoint(text string, maxPos int) int {
	runes := []rune(text)
	if maxPos >= len(runes) {
		return -1
	}
	// 从后往前找最近的换行
	for i := maxPos - 1; i > maxPos/2; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	// 从后往前找最近的句号或空格
	for i := maxPos - 1; i > maxPos/2; i-- {
		ch := runes[i]
		if ch == '。' || ch == '.' || ch == ' ' || ch == '，' || ch == ',' {
			return i + 1
		}
	}
	return -1
}

// ExtractKeywords 从文本中提取简单关键词（按空格和标点分割，取前 N 个）。
func ExtractKeywords(text string, topN int) string {
	if text == "" {
		return ""
	}

	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "up": true, "about": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "out": true, "off": true,
		"over": true, "under": true, "again": true, "further": true, "then": true,
		"once": true, "here": true, "there": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "every": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true,
		"only": true, "own": true, "same": true, "so": true, "than": true,
		"too": true, "very": true, "just": true, "because": true, "but": true,
		"and": true, "or": true, "if": true, "while": true, "this": true,
		"that": true, "it": true, "its": true, "he": true, "she": true,
		"they": true, "them": true, "their": true, "what": true, "which": true,
		"who": true, "whom": true, "my": true, "your": true, "his": true,
		"her": true, "our": true, "i": true, "me": true, "we": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "那": true, "他": true,
		"她": true, "它": true, "们": true, "来": true,
	}

	// 分词（简单按空白和中文分词）
	words := tokenize(text)
	wordCount := make(map[string]int)
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) < 2 || stopwords[w] {
			continue
		}
		wordCount[w]++
	}

	type kv struct {
		word string
		cnt  int
	}
	var ranked []kv
	for w, c := range wordCount {
		ranked = append(ranked, kv{w, c})
	}

	// 简单排序
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].cnt > ranked[i].cnt {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	var keywords []string
	for i, kv := range ranked {
		if i >= topN {
			break
		}
		keywords = append(keywords, kv.word)
	}
	return strings.Join(keywords, " ")
}

// tokenize 简单分词：按空格分割，同时提取中文连续字符。
func tokenize(text string) []string {
	text = strings.ToLower(text)
	// 替换标点为空格
	replacer := strings.NewReplacer(
		",", " ", ".", " ", ";", " ", ":", " ", "!", " ", "?", " ",
		"，", " ", "。", " ", "；", " ", "：", " ", "！", " ", "？", " ",
		"\n", " ", "\t", " ", "\r", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ", "\"", " ", "'", " ",
	)
	text = replacer.Replace(text)

	var words []string
	fields := strings.Fields(text)
	for _, f := range fields {
		if isChinese(f) {
			// 对中文做 2-gram
			for i := 0; i < len(f)-1; i++ {
				words = append(words, f[i:i+2])
			}
		} else {
			words = append(words, f)
		}
	}
	return words
}

// isChinese 判断字符串是否包含中文字符。
func isChinese(s string) bool {
	for _, r := range s {
		if r >= '一' && r <= '鿿' {
			return true
		}
	}
	return false
}
