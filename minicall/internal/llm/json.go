package llm

import (
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

// Ptr 快速生成 T 类型值的指针，简化字面量构造指针字段
// v：任意基础/结构体类型值
// 返回值：入参 v 的指针 *T
// 解决痛点：结构体赋值时需要指针字段，原生需拆分临时变量再取地址，一行简化代码
func Ptr[T any](v T) *T {
	return &v
}
