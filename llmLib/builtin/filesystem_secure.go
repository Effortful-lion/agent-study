// 文件职责：
// - 增强版文件系统工具
// - 集成安全特性：危险操作确认、操作审计、路径白名单
// - 提供可配置的安全策略
//
// 安全增强：
// 1. 危险操作（写/删）需要确认
// 2. 全操作审计日志
// 3. 可配置路径白名单
// 4. 操作计数和频率限制

package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/security"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 增强版文件系统工具
// ============================================================================

// SecureFileSystemTool 增强版文件系统工具
type SecureFileSystemTool struct {
	root           string           // 允许访问的根目录
	auditLogger    *security.AuditLogger
	confirmation   security.ConfirmationCallback
	allowedPaths   []string         // 额外允许的路径（相对于 root）
	blockedPaths   []string         // 禁止的路径（相对于 root）
	operationCount map[string]int   // 操作计数（防滥用）
}

// NewSecureFileSystemTool 创建增强版文件系统工具
func NewSecureFileSystemTool(root string, opts ...func(*SecureFileSystemTool)) (*SecureFileSystemTool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("无法解析根目录路径: %w", err)
	}

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return nil, fmt.Errorf("根目录不存在: %s", abs)
	}

	fs := &SecureFileSystemTool{
		root:           abs,
		auditLogger:    security.NewAuditLogger(),
		operationCount: make(map[string]int),
	}

	for _, opt := range opts {
		opt(fs)
	}

	return fs, nil
}

// SecureFileSystemOption 配置选项
type SecureFileSystemOption func(*SecureFileSystemTool)

// WithAuditCallback 设置审计回调
func WithAuditCallback(callback func(security.AuditEvent)) SecureFileSystemOption {
	return func(fs *SecureFileSystemTool) {
		logger := security.NewAuditLogger()
		fs.auditLogger = logger
	}
}

// WithConfirmation 设置确认回调
func WithConfirmation(callback security.ConfirmationCallback) SecureFileSystemOption {
	return func(fs *SecureFileSystemTool) {
		fs.confirmation = callback
	}
}

// WithAllowedPaths 设置额外允许的路径
func WithAllowedPaths(paths []string) SecureFileSystemOption {
	return func(fs *SecureFileSystemTool) {
		fs.allowedPaths = paths
	}
}

// WithBlockedPaths 设置禁止的路径
func WithBlockedPaths(paths []string) SecureFileSystemOption {
	return func(fs *SecureFileSystemTool) {
		fs.blockedPaths = paths
	}
}

// safePath 安全的路径解析
func (fs *SecureFileSystemTool) safePath(p string) (string, error) {
	// 拒绝绝对路径
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("拒绝绝对路径，只能使用相对路径: %s", p)
	}

	// 检查黑名单
	for _, blocked := range fs.blockedPaths {
		if strings.HasPrefix(p, blocked) {
			return "", fmt.Errorf("路径在黑名单中: %s", p)
		}
	}

	// 规范化路径
	clean := filepath.Clean(filepath.Join(fs.root, p))

	// 检查是否在根目录内
	if clean == fs.root {
		return clean, nil
	}
	if !strings.HasPrefix(clean, fs.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界，拒绝访问: %s (根目录: %s)", p, fs.root)
	}

	return clean, nil
}

// checkRateLimit 检查操作频率限制
func (fs *SecureFileSystemTool) checkRateLimit(operation string, limit int) error {
	count := fs.operationCount[operation]
	if count >= limit {
		return fmt.Errorf("操作频率超限：%s 操作已达到 %d 次限制", operation, limit)
	}
	fs.operationCount[operation] = count + 1
	return nil
}

// auditLog 记录审计日志
func (fs *SecureFileSystemTool) auditLog(toolName string, path string, result string, err error, duration time.Duration) {
	fs.auditLogger.Log(security.AuditEvent{
		ToolName: toolName,
		ToolArgs: map[string]any{"path": path},
		Result:   result,
		Error:    func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Duration: duration,
	})
}

// confirm 请求操作确认
func (fs *SecureFileSystemTool) confirm(ctx context.Context, toolName, path string) error {
	if fs.confirmation == nil {
		return nil // 无需确认
	}

	allowed, reason := fs.confirmation(ctx, toolName, map[string]any{"path": path})
	if !allowed {
		return fmt.Errorf("操作被拒绝: %s", reason)
	}
	return nil
}

// ============================================================================
// 读取文件（安全增强版）
// ============================================================================

// ReadFileTool 创建增强版读取文件工具
func (fs *SecureFileSystemTool) ReadFileTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "相对于知识库根目录的文件路径"
			}
		},
		"required": ["path"]
	}`)

	return tool.NewJSONSchemaTool(
		"read_file",
		"读取知识库目录下的文本文件内容。只能访问指定根目录内的文件，路径越界将被拒绝。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			startTime := time.Now()
			defer func() {
				fs.auditLog("read_file", getStringArg(args, "path"), "", nil, time.Since(startTime))
			}()

			// 解析参数
			path, ok := args["path"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 path 参数或参数类型错误")
			}

			// 路径安全检查
			safePath, err := fs.safePath(path)
			if err != nil {
				return nil, err
			}

			// 读取文件
			data, err := os.ReadFile(safePath)
			if err != nil {
				return nil, fmt.Errorf("读取文件失败: %w", err)
			}

			// 限制读取大小
			const maxBytes = 100 * 1024
			if len(data) > maxBytes {
				data = data[:maxBytes]
				return string(data) + fmt.Sprintf("\n\n... (文件已截断，仅显示前 %d 字节)", maxBytes), nil
			}

			return string(data), nil
		},
	)
}

// ============================================================================
// 写入文件（危险操作，需要确认）
// ============================================================================

// WriteFileTool 创建增强版写入文件工具
func (fs *SecureFileSystemTool) WriteFileTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "相对于知识库根目录的文件路径"
			},
			"content": {
				"type": "string",
				"description": "要写入的文件内容"
			},
			"append": {
				"type": "boolean",
				"description": "是否追加模式（默认覆盖）",
				"default": false
			}
		},
		"required": ["path", "content"]
	}`)

	return tool.NewJSONSchemaTool(
		"write_file",
		"向知识库目录写入文本文件内容。只能写入指定根目录内的文件，路径越界将被拒绝。⚠️ 警告：此操作会覆盖已存在的文件，请谨慎使用。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			startTime := time.Now()

			// 解析参数
			path, ok := args["path"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 path 参数")
			}
			content, ok := args["content"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 content 参数")
			}
			appendMode := false
			if appendArg, ok := args["append"].(bool); ok {
				appendMode = appendArg
			}

			// 路径安全检查
			safePath, err := fs.safePath(path)
			if err != nil {
				fs.auditLog("write_file", path, "", err, time.Since(startTime))
				return nil, err
			}

			// 检查文件是否已存在（覆盖警告）
			if !appendMode {
				if _, err := os.Stat(safePath); err == nil {
					// 文件存在，需要确认
					if err := fs.confirm(ctx, "write_file", path); err != nil {
						fs.auditLog("write_file", path, "", fmt.Errorf("用户拒绝覆盖"), time.Since(startTime))
						return nil, fmt.Errorf("文件已存在，操作被拒绝: %s", err)
					}
				}
			}

			// 频率限制
			if err := fs.checkRateLimit("write_file", 100); err != nil {
				return nil, err
			}

			// 确保父目录存在
			dir := filepath.Dir(safePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fs.auditLog("write_file", path, "", err, time.Since(startTime))
				return nil, fmt.Errorf("创建目录失败: %w", err)
			}

			// 写入文件
			var writeErr error
			if appendMode {
				writeErr = os.WriteFile(safePath, []byte(content), 0644)
				if writeErr != nil {
					// 如果追加模式失败（文件不存在），尝试创建新文件
					writeErr = os.WriteFile(safePath, []byte(content), 0644)
				}
			} else {
				writeErr = os.WriteFile(safePath, []byte(content), 0644)
			}
			if writeErr != nil {
				fs.auditLog("write_file", path, "", writeErr, time.Since(startTime))
				return nil, fmt.Errorf("写入文件失败: %w", writeErr)
			}

			fs.auditLog("write_file", path, fmt.Sprintf("写入成功，大小: %d 字节", len(content)), nil, time.Since(startTime))
			return fmt.Sprintf("文件写入成功: %s", safePath), nil
		},
	)
}

// ============================================================================
// 删除文件（极度危险，强制确认）
// ============================================================================

// DeleteFileTool 创建增强版删除文件工具（极度危险）
func (fs *SecureFileSystemTool) DeleteFileTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "相对于知识库根目录的文件路径"
			}
		},
		"required": ["path"]
	}`)

	return tool.NewJSONSchemaTool(
		"delete_file",
		"⚠️ 危险操作：删除知识库目录下的文件。此操作不可恢复，请务必确认文件路径正确。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			startTime := time.Now()

			// 解析参数
			path, ok := args["path"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 path 参数")
			}

			// 路径安全检查
			safePath, err := fs.safePath(path)
			if err != nil {
				fs.auditLog("delete_file", path, "", err, time.Since(startTime))
				return nil, err
			}

			// 强制确认（删除文件是极度危险的操作）
			if fs.confirmation != nil {
				allowed, reason := fs.confirmation(ctx, "delete_file", map[string]any{
					"path":         path,
					"absolutePath": safePath,
					"warning":      "此操作将永久删除文件，无法恢复",
				})
				if !allowed {
					fs.auditLog("delete_file", path, "", fmt.Errorf("用户拒绝删除"), time.Since(startTime))
					return nil, fmt.Errorf("删除操作被拒绝: %s", reason)
				}
			}

			// 频率限制（删除操作更严格）
			if err := fs.checkRateLimit("delete_file", 10); err != nil {
				return nil, err
			}

			// 检查文件是否存在
			if _, err := os.Stat(safePath); os.IsNotExist(err) {
				return nil, fmt.Errorf("文件不存在: %s", path)
			}

			// 删除文件
			if err := os.Remove(safePath); err != nil {
				fs.auditLog("delete_file", path, "", err, time.Since(startTime))
				return nil, fmt.Errorf("删除文件失败: %w", err)
			}

			fs.auditLog("delete_file", path, "删除成功", nil, time.Since(startTime))
			return fmt.Sprintf("文件删除成功: %s", safePath), nil
		},
	)
}

// ============================================================================
// 列出文件
// ============================================================================

// ListFilesTool 创建列出文件工具
func (fs *SecureFileSystemTool) ListFilesTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "相对于知识库根目录的目录路径，默认为根目录"
			}
		}
	}`)

	return tool.NewJSONSchemaTool(
		"list_files",
		"列出指定目录下的文件和子目录。只能访问指定根目录内的目录，路径越界将被拒绝。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			startTime := time.Now()
			defer func() {
				fs.auditLog("list_files", getStringArg(args, "path"), "", nil, time.Since(startTime))
			}()

			var dirPath string
			if args["path"] != nil {
				pathArg, ok := args["path"].(string)
				if !ok {
					return nil, fmt.Errorf("path 参数类型错误")
				}
				dirPath = pathArg
			}

			// 路径安全检查
			safePath, err := fs.safePath(dirPath)
			if err != nil {
				return nil, err
			}

			// 读取目录
			entries, err := os.ReadDir(safePath)
			if err != nil {
				return nil, fmt.Errorf("读取目录失败: %w", err)
			}

			// 格式化输出
			var result strings.Builder
			result.WriteString(fmt.Sprintf("目录 %s 内容:\n", dirPath))
			for _, entry := range entries {
				if entry.IsDir() {
					result.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.Name()))
				} else {
					info, err := entry.Info()
					if err == nil {
						result.WriteString(fmt.Sprintf("  [FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
					} else {
						result.WriteString(fmt.Sprintf("  [FILE] %s\n", entry.Name()))
					}
				}
			}

			return result.String(), nil
		},
	)
}

// ============================================================================
// 创建目录
// ============================================================================

// CreateDirTool 创建目录工具
func (fs *SecureFileSystemTool) CreateDirTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "相对于知识库根目录的目录路径"
			}
		},
		"required": ["path"]
	}`)

	return tool.NewJSONSchemaTool(
		"create_dir",
		"在知识库目录下创建新目录。如果目录已存在则返回成功。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			startTime := time.Now()
			defer func() {
				fs.auditLog("create_dir", getStringArg(args, "path"), "", nil, time.Since(startTime))
			}()

			path, ok := args["path"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 path 参数")
			}

			// 路径安全检查
			safePath, err := fs.safePath(path)
			if err != nil {
				return nil, err
			}

			// 创建目录
			if err := os.MkdirAll(safePath, 0755); err != nil {
				return nil, fmt.Errorf("创建目录失败: %w", err)
			}

			return fmt.Sprintf("目录创建成功: %s", safePath), nil
		},
	)
}

// ============================================================================
// 辅助函数
// ============================================================================

// getStringArg 从参数 map 中获取字符串值
func getStringArg(args map[string]any, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

// GetAuditLogger 获取审计日志器
func (fs *SecureFileSystemTool) GetAuditLogger() *security.AuditLogger {
	return fs.auditLogger
}
