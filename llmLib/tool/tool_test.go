package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Effortful-lion/agent-study/llmLib/core"
)

func TestToolDefsSchema(t *testing.T) {
	r := NewRegistryToolSet()
	r.Register(&CalculatorTool{})
	r.Register(&TimeTool{})

	defs := r.ToolDefs()
	if len(defs) != 2 {
		t.Fatalf("expected 2 tool defs, got %d", len(defs))
	}

	for _, def := range defs {
		var schema map[string]any
		if err := json.Unmarshal(def.Function.Parameters, &schema); err != nil {
			t.Errorf("tool %s: parameters is not valid JSON: %v (raw: %s)",
				def.Function.Name, err, string(def.Function.Parameters))
			continue
		}
		if schema["type"] != "object" {
			t.Logf("tool %s: WARNING — parameters 缺少 type=object，不是合法的 JSON Schema。raw: %s",
				def.Function.Name, string(def.Function.Parameters))
		}
		if _, ok := schema["properties"]; !ok {
			t.Logf("tool %s: WARNING — parameters 缺少 properties 字段。raw: %s",
				def.Function.Name, string(def.Function.Parameters))
		}
	}
}

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

var _ = core.ToolDef{}