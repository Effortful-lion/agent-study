// 文件职责：
// - 提供最小化的 SSE 解析器，只关注 data 字段的拼装和分发。
// - 供 OpenAI、Claude 等流式接口在读取事件流时复用。

package llmlib

import (
	"bufio"
	"io"
	"strings"
)

// ParseSSE 逐行读取 SSE 流，并在事件结束时把 data 字段拼装后交给回调处理。
func ParseSSE(r io.Reader, onData func(data []byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataLines []string
	dispatch := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		return onData([]byte(data))
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			// 空行表示一个 SSE 事件结束，此时触发回调处理累积的 data 段。
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			field, value = line, ""
		} else {
			value = strings.TrimPrefix(value, " ")
		}
		if field == "data" {
			dataLines = append(dataLines, value)
		}
	}
	return scanner.Err()
}
