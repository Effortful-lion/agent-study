package llmlib

type Schema struct {
	Type        string             `json:"type,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Required    []string           `json:"required,omitempty"`
}

// Generate 为任意值的类型生成 JSON Schema。
// 实现是一段 reflect 递归：遍历字段、按 kind 映射、读取 json/desc 标签。
func Generate(v any) *Schema {
	/* reflect 递归：string→"string"，struct→"object" ... */
	return nil
}
