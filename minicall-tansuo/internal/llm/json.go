package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ParseInto 将原始JSON字符串 raw 反序列化为目标泛型类型 T 对象
// raw：待解析的JSON文本字符串
// 返回值：
//
//	第一个返回值：解析成功后的 T 类型实例；解析失败时返回 T 零值
//	第二个返回值：解析失败时携带包装错误（包含目标类型+原始解析错误，支持 errors.Is/As）
//
// 特性：使用泛型无需手动声明中间变量，统一封装JSON反序列化逻辑，集中包装错误信息
func ParseInto[T any](raw string) (T, error) {
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return v, fmt.Errorf("parse into %T failed: %w", v, err)
	}
	return v, nil
}

// ParseRawInto 延迟解析 json.RawMessage，适合先保存工具调用参数原始字节，
// 等确认工具类型后再解析成具体参数结构。
func ParseRawInto[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("parse raw into %T failed: %w", v, err)
	}
	return v, nil
}

// ParseStrict 使用 json.Decoder + DisallowUnknownFields 做严格解析。
// 它适合测试、调试、内部强约束协议；生产解析通常不要开启，避免厂商扩展字段导致兼容性问题。
func ParseStrict[T any](raw string) (T, error) {
	var v T
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil {
		return v, fmt.Errorf("strict parse into %T failed: %w", v, err)
	}
	return v, nil
}

// SafeMarshal 安全序列化 JSON，调用方通过 error 处理失败，避免在序列化失败时 panic。
func SafeMarshal(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal %T failed: %w", v, err)
	}
	return data, nil
}

// ToolCall 表示大模型工具调用。Args 保留原始 JSON 字节，等知道具体工具类型后再延迟解析。
type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"arguments"`
}

// ContentPart 是多模态消息的一个分块，同一个 content 字段可能包含文字、图片、视频等结构。
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
}

// MediaURL 描述图片、视频等 URL 型多模态资源。
type MediaURL struct {
	URL string `json:"url"`
}

// MessageContent 兼容 content 字段的两种形态：纯文本字符串或多模态分块数组。
type MessageContent struct {
	Text  string
	Parts []ContentPart
}

func (m *MessageContent) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		m.Text = text
		m.Parts = nil
		return nil
	}

	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		m.Text = ""
		m.Parts = parts
		return nil
	}

	return fmt.Errorf("content is neither string nor parts array: %s", data)
}

func (m MessageContent) MarshalJSON() ([]byte, error) {
	if len(m.Parts) > 0 {
		return SafeMarshal(m.Parts)
	}
	return SafeMarshal(m.Text)
}

// Ptr 快速生成 T 类型值的指针，简化字面量构造指针字段
// v：任意基础/结构体类型值
// 返回值：入参 v 的指针 *T
// 解决痛点：结构体赋值时需要指针字段，原生需拆分临时变量再取地址，一行简化代码
func Ptr[T any](v T) *T {
	return &v
}
