// 文件职责：
// - 定义 Tool 接口和工具注册机制，支持 Agent 调用外部工具。
// - Tool 接口包含名称、描述、参数和调用方法。
// - Registry 管理工具集合，提供工具定义获取和工具查找功能。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// Tool 接口定义 Agent 可调用的工具，实现此接口即可被 Agent 使用。
// 工具是 Agent 的感知和行动接口，通过工具调用外部服务和能力。
type Tool interface {
	Name() string                                               // 工具名称，用于模型识别和调用
	Description() string                                        // 工具描述，用于模型理解工具用途
	Parameters() map[string]string                              // 参数描述，key 为参数名，value 为参数类型和说明
	Call(ctx context.Context, args map[string]any) (any, error) // 执行工具调用
}

// Registry 是工具注册表，管理 Agent 可用的所有工具。
// 支持工具的注册、查找和工具定义列表生成。
type Registry struct {
	tools map[string]Tool
}

// NewRegistryToolSet 创建一个新的工具注册表。
func NewRegistryToolSet() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register 注册一个工具到注册表。
// 如果同名工具已存在，将被覆盖。
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get 根据工具名称查找工具。
// 返回工具和是否找到。
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// ToolDefs 返回所有已注册工具的定义列表，用于传递给模型。
func (r *Registry) ToolDefs() []ToolDef {
	var defs []ToolDef
	for _, tool := range r.tools {
		params, _ := json.Marshal(tool.Parameters())
		defs = append(defs, ToolDef{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  params,
		})
	}
	return defs
}

// Call 调用指定名称的工具，自动解析参数。
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, NewAgentError(ErrCategoryToolNotFound, fmt.Sprintf("工具 %s 不存在", name), nil, false)
	}
	return tool.Call(ctx, args)
}

// BuildArgs 将 JSON 字符串参数转换为 map[string]any。
func BuildArgs(argsJSON string) (map[string]any, error) {
	if argsJSON == "" {
		return nil, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, NewAgentError(ErrCategoryTool, "参数解析失败: "+err.Error(), err, false)
	}
	return args, nil
}

// StructToMap 将结构体转换为 map[string]any，用于工具参数传递。
func StructToMap(v any) map[string]any {
	result := make(map[string]any)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return result
	}
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		name := typ.Field(i).Name
		if !field.CanInterface() {
			continue
		}
		result[name] = field.Interface()
	}
	return result
}
