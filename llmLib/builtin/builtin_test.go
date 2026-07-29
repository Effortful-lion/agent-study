// 文件职责：
// - 测试文件系统工具和 NL2SQL 工具的安全防护
// - 验证路径围栏是否正常工作
// - 验证 SQL 关键字过滤是否有效

package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ========== 文件系统工具测试 ==========

func TestFileSystemTool_PathJail(t *testing.T) {
	// 创建临时目录作为根目录
	rootDir, err := os.MkdirTemp("", "fs_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(rootDir)

	fs, err := NewFileSystemTool(rootDir)
	if err != nil {
		t.Fatalf("创建 FileSystemTool 失败: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{"当前目录文件", "test.txt", false},
		{"子目录文件", "subdir/file.txt", false},
		{"路径遍历攻击", "../../etc/passwd", true},
		{"系统文件路径", "/etc/passwd", true},
		{"深度路径遍历", "../../../etc/shadow", true},
		{"空路径（根目录）", "", false},
		{"点路径（当前目录）", ".", false},
		{"双点路径（父目录）", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fs.safePath(tt.path)
			if tt.expectErr && err == nil {
				t.Errorf("期望拒绝路径 %q，但成功了", tt.path)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("期望接受路径 %q，但被拒绝: %v", tt.path, err)
			}
		})
	}
}

func TestFileSystemTool_ReadFileTool(t *testing.T) {
	// 创建临时目录和测试文件
	rootDir, err := os.MkdirTemp("", "fs_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// 创建测试文件
	testContent := "Hello, World!"
	testFile := filepath.Join(rootDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 创建大文件测试截断
	largeContent := string(make([]byte, 200*1024)) // 200KB
	largeFile := filepath.Join(rootDir, "large.txt")
	if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("创建大文件失败: %v", err)
	}

	fs, err := NewFileSystemTool(rootDir)
	if err != nil {
		t.Fatalf("创建 FileSystemTool 失败: %v", err)
	}

	registry := tool.NewRegistryToolSet()
	registry.Register(fs.ReadFileTool())

	t.Run("读取正常文件", func(t *testing.T) {
		args := map[string]any{"path": "test.txt"}
		result, err := registry.Call(context.Background(), "read_file", args)
		if err != nil {
			t.Fatalf("读取文件失败: %v", err)
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Fatalf("期望字符串结果，得到 %T", result)
		}
		if resultStr != testContent {
			t.Errorf("文件内容不匹配。期望 %q，得到 %q", testContent, resultStr)
		}
	})

	t.Run("读取大文件截断", func(t *testing.T) {
		args := map[string]any{"path": "large.txt"}
		result, err := registry.Call(context.Background(), "read_file", args)
		if err != nil {
			t.Fatalf("读取文件失败: %v", err)
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Fatalf("期望字符串结果，得到 %T", result)
		}

		// 检查是否被截断
		if len(resultStr) != 100*1024+52 { // 100KB + 截断提示
			t.Errorf("期望截断到 %d 字节，实际 %d 字节", 100*1024+52, len(resultStr))
		}

		if !contains(resultStr, "... (文件已截断") {
			t.Error("缺少截断提示")
		}
	})

	t.Run("读取不存在的文件", func(t *testing.T) {
		args := map[string]any{"path": "nonexistent.txt"}
		_, err := registry.Call(context.Background(), "read_file", args)
		if err == nil {
			t.Error("期望文件不存在错误，但没有返回错误")
		}
	})

	t.Run("路径遍历攻击", func(t *testing.T) {
		args := map[string]any{"path": "../../etc/passwd"}
		_, err := registry.Call(context.Background(), "read_file", args)
		if err == nil {
			t.Error("路径遍历攻击应该被拒绝，但成功了")
		}

		// 验证错误消息包含越界提示
		if err != nil && !contains(err.Error(), "越界") {
			t.Errorf("错误消息应包含 '越界' 提示: %v", err)
		}
	})
}

func TestFileSystemTool_WriteFileTool(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "fs_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(rootDir)

	fs, err := NewFileSystemTool(rootDir)
	if err != nil {
		t.Fatalf("创建 FileSystemTool 失败: %v", err)
	}

	registry := tool.NewRegistryToolSet()
	registry.Register(fs.WriteFileTool())

	t.Run("写入新文件", func(t *testing.T) {
		args := map[string]any{
			"path":    "newfile.txt",
			"content": "test content",
		}
		result, err := registry.Call(context.Background(), "write_file", args)
		if err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Fatalf("期望字符串结果，得到 %T", result)
		}
		if !contains(resultStr, "成功") {
			t.Error("缺少成功提示")
		}

		// 验证文件确实被创建
		filePath := filepath.Join(rootDir, "newfile.txt")
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("读取新文件失败: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("文件内容不匹配")
		}
	})

	t.Run("写入子目录自动创建", func(t *testing.T) {
		args := map[string]any{
			"path":    "subdir/nested/file.txt",
			"content": "nested content",
		}
		_, err := registry.Call(context.Background(), "write_file", args)
		if err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}

		// 验证文件被创建
		filePath := filepath.Join(rootDir, "subdir/nested/file.txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("文件未被创建")
		}
	})

	t.Run("路径遍历攻击", func(t *testing.T) {
		args := map[string]any{
			"path":    "../../etc/evil.txt",
			"content": "malicious",
		}
		_, err := registry.Call(context.Background(), "write_file", args)
		if err == nil {
			t.Error("路径遍历攻击应该被拒绝，但成功了")
		}
	})
}

// ========== NL2SQL 工具测试 ==========

func TestNL2SQL_IsSelectOnly(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		expectErr bool
	}{
		{"正常 SELECT", "SELECT * FROM users", false},
		{"带 WHERE 的 SELECT", "SELECT id, name FROM users WHERE age > 18", false},
		{"带 JOIN 的 SELECT", "SELECT * FROM users JOIN orders ON users.id = orders.user_id", false},
		{"带子查询的 SELECT", "SELECT * FROM (SELECT id FROM users) t", false},
		{"末尾带分号", "SELECT * FROM users;", false},
		{"INSERT 语句", "INSERT INTO users (name) VALUES ('test')", true},
		{"UPDATE 语句", "UPDATE users SET name = 'test'", true},
		{"DELETE 语句", "DELETE FROM users WHERE id = 1", true},
		{"DROP 语句", "DROP TABLE users", true},
		{"多条语句", "SELECT * FROM users; DELETE FROM users", true},
		{"UNION 注入", "SELECT * FROM users UNION SELECT * FROM passwords", true},
		{"注释注入", "SELECT * FROM users -- comment", true},
		{"以小写开头", "select * from users", false},
		{"空语句", "", true},
		{"仅空格", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isSelectOnly(tt.sql)
			if tt.expectErr && err == nil {
				t.Errorf("期望 SQL %q 被拒绝，但成功了", tt.sql)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("期望 SQL %q 被接受，但被拒绝: %v", tt.sql, err)
			}
		})
	}
}

// 注意：NL2SQL 工具的完整测试需要真实的数据库连接
// 这里提供一个简单的单元测试示例

func TestNL2SQL_KeywordFilter(t *testing.T) {
	// 测试关键字过滤是否生效
	dangerousSQLs := []string{
		"DROP TABLE users",
		"DELETE FROM users",
		"UPDATE users SET admin=1",
		"INSERT INTO users VALUES ('hacker')",
		"TRUNCATE TABLE logs",
		"GRANT ALL ON *.* TO 'hacker'@'%'",
		"EXEC xp_cmdshell('whoami')",
	}

	for _, sql := range dangerousSQLs {
		err := isSelectOnly(sql)
		if err == nil {
			t.Errorf("危险 SQL 应该被拒绝: %s", sql)
		}
	}

	safeSQLs := []string{
		"SELECT * FROM users",
		"SELECT COUNT(*) FROM orders",
		"SELECT name FROM users WHERE id = 1",
	}

	for _, sql := range safeSQLs {
		err := isSelectOnly(sql)
		if err != nil {
			t.Errorf("安全 SELECT 应该被接受: %s, 错误: %v", sql, err)
		}
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
