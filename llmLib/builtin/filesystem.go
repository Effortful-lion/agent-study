// 文件职责：
// - 实现带路径围栏（path jail）防护的文件系统工具
// - 确保所有文件访问都限制在指定根目录内，防止路径遍历攻击
// - 提供 read_file 工具，支持读取文本文件并限制读取大小

package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// FileSystemTool 是带路径围栏的文件系统工具
// root 是允许访问的根目录（绝对路径），所有文件访问都必须在此目录内
type FileSystemTool struct {
	root string // 允许访问的根目录（绝对路径）
}

// NewFileSystemTool 创建一个新的文件系统工具
// root: 允许访问的根目录路径
func NewFileSystemTool(root string) (*FileSystemTool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("无法解析根目录路径: %w", err)
	}
	// 确保根目录存在
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return nil, fmt.Errorf("根目录不存在: %s", abs)
	}
	return &FileSystemTool{root: abs}, nil
}

// safePath 将相对路径解析为根目录内的绝对路径，越界则报错
// 这是路径围栏的核心：防止通过 ../etc/passwd 等路径遍历访问系统文件
func (fs *FileSystemTool) safePath(p string) (string, error) {
	// 拒绝绝对路径（防止绕过路径围栏）
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("拒绝绝对路径，只能使用相对路径: %s", p)
	}

	// 使用 filepath.Join 和 filepath.Clean 规范化路径
	// filepath.Join 会自动处理相对路径
	clean := filepath.Clean(filepath.Join(fs.root, p))

	// 检查规范化后的路径是否仍在根目录内
	// 两种情况允许：1) 就是根目录本身 2) 以根目录+路径分隔符开头
	if clean == fs.root {
		return clean, nil
	}
	if !strings.HasPrefix(clean, fs.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界，拒绝访问: %s (根目录: %s)", p, fs.root)
	}
	return clean, nil
}

// readFileArgs 是 read_file 工具的参数字段
type readFileArgs struct {
	Path string `json:"path" desc:"相对于知识库根目录的文件路径"`
}

// ReadFileTool 创建读取文件的工具
// 该工具受路径围栏保护，只能读取指定根目录内的文件
func (fs *FileSystemTool) ReadFileTool() tool.Tool {
	// 构建 JSON Schema
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
			// 解析参数
			path, ok := args["path"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 path 参数或参数类型错误")
			}

			// 路径安全检查（路径围栏）
			safePath, err := fs.safePath(path)
			if err != nil {
				return nil, err
			}

			// 读取文件
			data, err := os.ReadFile(safePath)
			if err != nil {
				return nil, fmt.Errorf("读取文件失败: %w", err)
			}

			// 限制读取大小（防止读取超大文件撑爆上下文）
			const maxBytes = 100 * 1024 // 100KB
			if len(data) > maxBytes {
				data = data[:maxBytes]
				return string(data) + fmt.Sprintf("\n\n... (文件已截断，仅显示前 %d 字节)", maxBytes), nil
			}

			return string(data), nil
		},
	)
}

// WriteFileTool 创建写入文件的工具（可选功能，同样受路径围栏保护）
type writeFileArgs struct {
	Path     string `json:"path" desc:"相对于知识库根目录的文件路径"`
	Content  string `json:"content" desc:"要写入的文件内容"`
	Append   bool   `json:"append,omitempty" desc:"是否追加模式（默认覆盖）"`
}

// WriteFileTool 创建写入文件的工具
func (fs *FileSystemTool) WriteFileTool() tool.Tool {
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
		"向知识库目录写入文本文件内容。只能写入指定根目录内的文件，路径越界将被拒绝。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
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
				return nil, err
			}

			// 确保父目录存在
			dir := filepath.Dir(safePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
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
				return nil, fmt.Errorf("写入文件失败: %w", writeErr)
			}

			return fmt.Sprintf("文件写入成功: %s", safePath), nil
		},
	)
}

// ListFilesTool 创建列出目录文件的工具
type listFilesArgs struct {
	Path string `json:"path,omitempty" desc:"相对于知识库根目录的目录路径，默认为根目录"`
}

// ListFilesTool 创建列出目录文件的工具
func (fs *FileSystemTool) ListFilesTool() tool.Tool {
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
