package tool

import (
	"reflect"
	"strings"
)

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
}

// Generate 传入任意变量（值/指针）生成JSON Schema
func Generate(v any) *Schema {
	if v == nil {
		return nil
	}
	return generateType(reflect.TypeOf(v), make(map[reflect.Type]bool))
}

// generateType 递归核心，visited防止循环引用
func generateType(typ reflect.Type, visited map[reflect.Type]bool) *Schema {
	// 解指针
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	s := &Schema{}
	switch typ.Kind() {
	case reflect.String:
		s.Type = "string"
	case reflect.Bool:
		s.Type = "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s.Type = "integer"
	case reflect.Float32, reflect.Float64:
		s.Type = "number"
	case reflect.Struct:
		if visited[typ] {
			return &Schema{Type: "object"} // 循环引用截断
		}
		visited[typ] = true
		s.Type = "object"
		s.Properties = make(map[string]*Schema)
		noExtra := false
		s.AdditionalProperties = &noExtra // 禁止额外字段，LLM场景关键

		for f := range typ.Fields() {
			jsonTag := f.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}
			tagParts := strings.Split(jsonTag, ",")
			fieldName := tagParts[0]
			if fieldName == "" {
				fieldName = f.Name
			}
			omitEmpty := false
			for _, p := range tagParts[1:] {
				if p == "omitempty" {
					omitEmpty = true
				}
			}

			subSchema := generateType(f.Type, visited)
			subSchema.Description = f.Tag.Get("desc")
			s.Properties[fieldName] = subSchema

			if !omitEmpty {
				s.Required = append(s.Required, fieldName)
			}
		}
	case reflect.Slice, reflect.Array:
		s.Type = "array"
		s.Items = generateType(typ.Elem(), visited)
	case reflect.Map:
		s.Type = "object"
		// map简单处理，可扩展additionalProperties
	default:
		s.Type = "string" // fallback兜底
	}
	return s
}
