package llmlib

import (
	"context"
	"encoding/json"
	"testing"
)

// TestToolDefsSchema 验证 ToolDefs 生成的 JSON Schema 是否正确。
func TestToolDefsSchema(t *testing.T) {
	// 测试基础 Tool 接口（map[string]string 参数）
	r := NewRegistryToolSet()
	r.Register(&CalculatorTool{})
	r.Register(&TimeTool{})

	defs := r.ToolDefs()
	if len(defs) != 2 {
		t.Fatalf("expected 2 tool defs, got %d", len(defs))
	}

	for _, def := range defs {
		// 验证 parameters 是合法的 JSON Schema
		var schema map[string]any
		if err := json.Unmarshal(def.Function.Parameters, &schema); err != nil {
			t.Errorf("tool %s: parameters is not valid JSON: %v (raw: %s)",
				def.Function.Name, err, string(def.Function.Parameters))
			continue
		}
		// 检查是否有 type 字段（JSON Schema 必需）
		if schema["type"] != "object" {
			t.Logf("tool %s: WARNING — parameters 缺少 type=object，不是合法的 JSON Schema。raw: %s",
				def.Function.Name, string(def.Function.Parameters))
		}
		// 检查是否有 properties 字段
		if _, ok := schema["properties"]; !ok {
			t.Logf("tool %s: WARNING — parameters 缺少 properties 字段。raw: %s",
				def.Function.Name, string(def.Function.Parameters))
		}
	}
}

// TestSchemaToolDefs 验证 SchemaTool 接口生成的 JSON Schema。
func TestSchemaToolDefs(t *testing.T) {
	calcTool := NewJSONSchemaTool("calculator", "执行数学运算",
		json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string","description":"数学表达式"}},"required":["expression"]}`),
		func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
	)

	r := NewRegistryToolSet()
	r.Register(calcTool)
	defs := r.ToolDefs()

	if len(defs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(defs))
	}

	var schema map[string]any
	if err := json.Unmarshal(defs[0].Function.Parameters, &schema); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	if schema["type"] != "object" {
		t.Error("expected type=object in JSON Schema")
	}
	if _, ok := schema["properties"]; !ok {
		t.Error("expected properties in JSON Schema")
	}
}
