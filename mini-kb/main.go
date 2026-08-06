package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/agent"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/config"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/index"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/memory"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/tools"
	"github.com/Effortful-lion/agent-study/mini-kb/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "ask":
		runAsk(os.Args[2:])
	case "chat":
		runChat(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "sessions":
		runSessions(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("mini-kb - 本地知识库问答工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  mini-kb init             初始化存储目录")
	fmt.Println("  mini-kb ingest <dir>     扫描目录并建立索引")
	fmt.Println("  mini-kb ask <question>   单轮知识库问答")
	fmt.Println("  mini-kb chat             进入连续对话模式")
	fmt.Println("  mini-kb status           查看索引状态")
	fmt.Println("  mini-kb sessions         查看会话历史")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  mini-kb init")
	fmt.Println("  mini-kb ingest ./data")
	fmt.Println("  mini-kb ask \"Go语言的协程是什么\"")
	fmt.Println("  mini-kb chat")
}

// ── init ──────────────────────────────────────────────────────────────────────

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", "", "存储目录（默认: ~/.mini-kb）")
	_ = fs.Parse(args)

	cfg := config.DefaultConfig()
	if *dir != "" {
		cfg.StorageDir = *dir
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "配置无效: %v\n", err)
		os.Exit(1)
	}

	// 创建目录
	dirs := []string{
		cfg.StorageDir,
		filepath.Join(cfg.StorageDir, "sessions"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败 %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	// 写入默认配置
	cfgPath := filepath.Join(cfg.StorageDir, "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "写入配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("初始化完成，存储目录: %s\n", cfg.StorageDir)
	fmt.Printf("配置文件: %s\n", cfgPath)
	fmt.Println()
	fmt.Println("下一步:")
	fmt.Println("  1. 将文档放入 data/examples/ 目录")
	fmt.Println("  2. 运行: mini-kb ingest ./data")
}

// ── ingest ────────────────────────────────────────────────────────────────────

func runIngest(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: mini-kb ingest <docDir> [--dir <storageDir>]")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	storageDir := fs.String("dir", "", "存储目录")
	chunkSize := fs.Int("chunk-size", 0, "块大小")
	chunkOverlap := fs.Int("chunk-overlap", 0, "块重叠")
	_ = fs.Parse(args)

	docDir := fs.Arg(0)
	cfg := loadOrCreateConfig(*storageDir)
	if *chunkSize > 0 {
		cfg.ChunkSize = *chunkSize
	}
	if *chunkOverlap >= 0 {
		cfg.ChunkOverlap = *chunkOverlap
	}

	fmt.Printf("扫描目录: %s\n", docDir)
	fmt.Printf("块大小: %d, 重叠: %d\n", cfg.ChunkSize, cfg.ChunkOverlap)

	// 收集文档
	docs, err := collectDocuments(docDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "扫描失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("发现 %d 个文档\n", len(docs))
	if len(docs) == 0 {
		fmt.Println("没有找到 Markdown 或 TXT 文档")
		return
	}

	// 切分
	splitter := index.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	var docRecords []storage.DocumentRecord
	var chunkRecords []storage.ChunkRecord

	for _, doc := range docs {
		content, err := readFile(doc.FilePath)
		if err != nil {
			fmt.Printf("警告: 读取 %s 失败: %v\n", doc.FilePath, err)
			continue
		}
		chunks, err := splitter.Chunk(doc, content)
		if err != nil {
			fmt.Printf("警告: 切分 %s 失败: %v\n", doc.FilePath, err)
			continue
		}
		for i := range chunks {
			chunks[i].Keyword = index.ExtractKeywords(chunks[i].Content, 10)
		}

		docRecords = append(docRecords, storage.ToDocumentRecord(doc))
		for _, c := range chunks {
			chunkRecords = append(chunkRecords, storage.ToChunkRecord(&c))
		}
		fmt.Printf("  %s -> %d 个块\n", doc.Title, len(chunks))
	}

	// 持久化
	docStore := storage.NewDocumentStore(filepath.Join(cfg.StorageDir, "documents.json"))
	chunkStore := storage.NewChunkStore(filepath.Join(cfg.StorageDir, "chunks.json"))

	if err := docStore.Save(docRecords); err != nil {
		fmt.Fprintf(os.Stderr, "保存文档失败: %v\n", err)
		os.Exit(1)
	}
	if err := chunkStore.Save(chunkRecords); err != nil {
		fmt.Fprintf(os.Stderr, "保存块失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n索引完成: %d 个文档, %d 个块\n", len(docRecords), len(chunkRecords))
	fmt.Printf("存储位置: %s\n", cfg.StorageDir)
}

// ── ask ───────────────────────────────────────────────────────────────────────

func runAsk(args []string) {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	storageDir := fs.String("dir", "", "存储目录")
	provider := fs.String("provider", "openai", "LLM 提供者")
	model := fs.String("model", "", "模型名称")
	apiKey := fs.String("api-key", "", "API 密钥")
	baseURL := fs.String("base-url", "", "API 端点")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "用法: mini-kb ask <question> [--provider openai] [--model gpt-4]")
		os.Exit(1)
	}
	question := fs.Arg(0)

	cfg := loadOrCreateConfig(*storageDir)
	answer, err := executeAsk(cfg, question, &agent.KBConfig{
		Provider: *provider,
		Model:    *model,
		APIKey:   *apiKey,
		BaseURL:  *baseURL,
		TopK:     cfg.TopK,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	printAnswer(answer)
}

// ── chat ──────────────────────────────────────────────────────────────────────

func runChat(args []string) {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	storageDir := fs.String("dir", "", "存储目录")
	provider := fs.String("provider", "openai", "LLM 提供者")
	model := fs.String("model", "", "模型名称")
	apiKey := fs.String("api-key", "", "API 密钥")
	baseURL := fs.String("base-url", "", "API 端点")
	sessionID := fs.String("session", "default", "会话 ID")
	_ = fs.Parse(args)

	cfg := loadOrCreateConfig(*storageDir)
	kb, err := buildKB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建知识库失败: %v\n", err)
		os.Exit(1)
	}

	a := agent.NewKnowledgeAgent(&agent.KBConfig{
		Provider: *provider,
		Model:    *model,
		APIKey:   *apiKey,
		BaseURL:  *baseURL,
		TopK:     cfg.TopK,
		MaxSteps: 10,
	}, kb, cfg.StorageDir, *sessionID)

	ctx := context.Background()
	if err := a.Chat(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// ── status ────────────────────────────────────────────────────────────────────

func runStatus(args []string) {
	cfg := loadOrCreateConfig("")

	docStore := storage.NewDocumentStore(filepath.Join(cfg.StorageDir, "documents.json"))
	chunkStore := storage.NewChunkStore(filepath.Join(cfg.StorageDir, "chunks.json"))

	docs, err := docStore.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载文档失败: %v\n", err)
		os.Exit(1)
	}
	chunks, err := chunkStore.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载块失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("存储目录: %s\n", cfg.StorageDir)
	fmt.Printf("文档数: %d\n", len(docs))
	fmt.Printf("文本块: %d\n", len(chunks))
	fmt.Println()
	if len(docs) > 0 {
		fmt.Println("已索引文档:")
		for _, d := range docs {
			fmt.Printf("  - %s (%s, %d 块)\n", d.Title, d.FilePath, countChunksForDoc(chunks, d.ID))
		}
	}
}

// ── sessions ──────────────────────────────────────────────────────────────────

func runSessions(args []string) {
	cfg := loadOrCreateConfig("")
	sessionDir := filepath.Join(cfg.StorageDir, "sessions")

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取会话目录失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("会话目录: %s\n", sessionDir)
	fmt.Printf("会话数: %d\n\n", len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(sessionDir, name))
		if err != nil {
			continue
		}
		var s memory.Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		fmt.Printf("会话: %s\n", s.ID)
		fmt.Printf("  创建: %s\n", s.CreatedAt)
		fmt.Printf("  更新: %s\n", s.UpdatedAt)
		fmt.Printf("  轮数: %d\n", len(s.Turns))
		if len(s.Turns) > 0 {
			last := s.Turns[len(s.Turns)-1]
			fmt.Printf("  最后问题: %s\n", truncate(last.Question, 60))
		}
		fmt.Println()
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func executeAsk(cfg *config.Config, question string, kbCfg *agent.KBConfig) (*agent.KBAnswer, error) {
	kb, err := buildKB(cfg)
	if err != nil {
		return nil, err
	}
	a := agent.NewKnowledgeAgent(kbCfg, kb, cfg.StorageDir, "ask-"+truncateID(question))
	ctx := context.Background()
	return a.Ask(ctx, question)
}

func buildKB(cfg *config.Config) (*tools.KBRetriever, error) {
	docStore := storage.NewDocumentStore(filepath.Join(cfg.StorageDir, "documents.json"))
	chunkStore := storage.NewChunkStore(filepath.Join(cfg.StorageDir, "chunks.json"))

	docs, err := docStore.Load()
	if err != nil {
		return nil, fmt.Errorf("加载文档失败: %w", err)
	}
	chunks, err := chunkStore.Load()
	if err != nil {
		return nil, fmt.Errorf("加载块失败: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("知识库为空，请先运行 'mini-kb ingest'")
	}

	var docPtrs []*document.Chunk
	for i := range chunks {
		chunks[i].DocumentID = resolveDocID(chunks[i].DocumentID, docs)
		docPtrs = append(docPtrs, &document.Chunk{
			ID:         chunks[i].ID,
			DocumentID: chunks[i].DocumentID,
			FilePath:   chunks[i].FilePath,
			Title:      chunks[i].Title,
			Content:    chunks[i].Content,
			StartPos:   chunks[i].StartPos,
			EndPos:     chunks[i].EndPos,
			Keyword:    chunks[i].Keyword,
		})
	}

	kb := tools.NewKBRetriever(cfg.StorageDir, cfg.TopK)
	kb.LoadChunks(docPtrs)
	return kb, nil
}

func resolveDocID(docID string, docs []storage.DocumentRecord) string {
	for _, d := range docs {
		if d.ID == docID {
			return docID
		}
	}
	// fallback: 用 file_path 作为 doc_id
	for _, d := range docs {
		if docID == d.FilePath {
			return d.ID
		}
	}
	return docID
}

func collectDocuments(dir string) ([]*document.Document, error) {
	var docs []*document.Document
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" && ext != ".markdown" {
			return nil
		}
		if info.Size() == 0 {
			return nil
		}
		doc, err := document.NewDocument(path)
		if err != nil {
			fmt.Printf("警告: 跳过 %s: %v\n", path, err)
			return nil
		}
		docs = append(docs, doc)
		return nil
	})
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].FilePath < docs[j].FilePath
	})
	return docs, nil
}

func readFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func loadOrCreateConfig(dir string) *config.Config {
	if dir != "" {
		cfg := config.DefaultConfig()
		cfg.StorageDir = dir
		return cfg
	}
	// 尝试从存储目录读取
	home, _ := os.UserHomeDir()
	storageDir := filepath.Join(home, ".mini-kb")
	cfgPath := filepath.Join(storageDir, "config.json")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg config.Config
		if json.Unmarshal(data, &cfg) == nil && cfg.Validate() == nil {
			return &cfg
		}
	}
	return config.DefaultConfig()
}

func printAnswer(answer *agent.KBAnswer) {
	if answer.Error != "" {
		fmt.Printf("[错误] %s\n\n", answer.Error)
	}
	if len(answer.Sources) > 0 {
		fmt.Println("来源:")
		for _, s := range answer.Sources {
			fmt.Printf("  - %s\n", s)
		}
		fmt.Println()
	}
	if len(answer.Tools) > 0 {
		fmt.Printf("使用工具: %s\n\n", strings.Join(answer.Tools, ", "))
	}
	fmt.Printf("回答:\n%s\n", answer.Answer)
}

func countChunksForDoc(chunks []storage.ChunkRecord, docID string) int {
	count := 0
	for _, c := range chunks {
		if c.DocumentID == docID {
			count++
		}
	}
	return count
}

func truncateID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 20 {
		s = s[:20]
	}
	// 替换文件名不安全字符
	replacer := strings.NewReplacer(
		" ", "_", "/", "_", "\\", "_", ":", "_",
		"?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
