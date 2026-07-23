// 文件职责：
// - 定义工具调用范式接口，支持多种工具调用格式的自动检测和解析。
// - FunctionCallingParadigm 解析结构化 JSON 工具调用（OpenAI 风格）。
// - ReActParadigm 解析文本格式工具调用（Action/Action Input 和 <function> 标签）。
// - DetectParadigm 根据响应内容自动检测使用哪种范式。

package llmlib

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCallingParadigm 接口定义工具调用范式的基本操作：检测和解析。
type ToolCallingParadigm interface {
	Detect(content string) bool               // 判断内容是否符合当前范式
	Parse(content string) ([]ToolCall, error) // 解析内容提取工具调用列表
}

// FunctionCallingParadigm 实现 Function Calling 范式，解析结构化 JSON 工具调用。
// 适用于支持原生工具调用的模型（如 OpenAI GPT-4、Anthropic Claude 3.5 等）。
type FunctionCallingParadigm struct{}

// Detect 判断响应内容是否包含结构化工具调用（JSON 格式）。
func (p *FunctionCallingParadigm) Detect(content string) bool {
	return strings.Contains(content, "\"tool_calls\"") ||
		strings.Contains(content, "\"function\"") ||
		strings.Contains(content, "\"name\"") && strings.Contains(content, "\"arguments\"")
}

// Parse 解析 JSON 格式的工具调用。
func (p *FunctionCallingParadigm) Parse(content string) ([]ToolCall, error) {
	var calls []ToolCall
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, NewAgentError(ErrCategoryModel, "工具调用格式解析失败", err, false)
	}
	if toolCalls, ok := data["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			if call, ok := tc.(map[string]interface{}); ok {
				name, _ := call["name"].(string)
				argsRaw, _ := call["arguments"].([]byte)
				if name != "" {
					calls = append(calls, ToolCall{Name: name, Args: argsRaw})
				}
			}
		}
	}
	if function, ok := data["function"].(map[string]interface{}); ok {
		name, _ := function["name"].(string)
		argsStr, _ := function["arguments"].(string)
		var args json.RawMessage
		if argsStr != "" {
			args = []byte(argsStr)
		}
		if name != "" {
			calls = append(calls, ToolCall{Name: name, Args: args})
		}
	}
	return calls, nil
}

// ReActParadigm 实现 ReAct 范式，解析文本格式的工具调用。
// 适用于不支持原生工具调用的模型，通过文本提示引导模型输出工具调用。
type ReActParadigm struct{}

// Detect 判断响应内容是否包含 ReAct 格式的工具调用。
func (p *ReActParadigm) Detect(content string) bool {
	return strings.Contains(content, "Action:") ||
		strings.Contains(content, "<function name=") ||
		strings.Contains(content, "[function")
}

// Parse 解析 ReAct 格式的工具调用，支持两种格式：
// 1. Action: tool_name\nAction Input: {"key": "value"}
// 2. <function name="tool_name">{"key": "value"}</function>
func (p *ReActParadigm) Parse(content string) ([]ToolCall, error) {
	var calls []ToolCall
	re := regexp.MustCompile(`Action:\s*(\w+)\s*\nAction Input:\s*(.+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		name := match[1]
		args := json.RawMessage(match[2])
		calls = append(calls, ToolCall{Name: name, Args: args})
	}
	re2 := regexp.MustCompile(`<function name="([^"]+)">(.+?)</function>`)
	matches2 := re2.FindAllStringSubmatch(content, -1)
	for _, match := range matches2 {
		name := match[1]
		args := json.RawMessage(match[2])
		calls = append(calls, ToolCall{Name: name, Args: args})
	}
	return calls, nil
}

// DetectParadigm 根据响应内容自动检测使用哪种工具调用范式。
// 优先检测 Function Calling 范式，其次检测 ReAct 范式。
func DetectParadigm(content string) ToolCallingParadigm {
	paradigms := []ToolCallingParadigm{
		&FunctionCallingParadigm{},
		&ReActParadigm{},
	}
	for _, p := range paradigms {
		if p.Detect(content) {
			return p
		}
	}
	return nil
}