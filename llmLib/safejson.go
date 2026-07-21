package llmlib

import (
	"encoding/json"
	"fmt"
)

func SafeJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(data)
}
