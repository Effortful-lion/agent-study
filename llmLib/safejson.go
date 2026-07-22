// 文件职责：
// - 提供安全的 JSON 序列化辅助函数。
// - 供日志、调试和错误输出场景在序列化失败时仍能得到可读文本。

package llmlib

import (
	"encoding/json"
	"fmt"
)

// SafeJSON 将任意值编码为 JSON 字符串，失败时返回携带错误信息的兜底 JSON。
func SafeJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(data)
}
