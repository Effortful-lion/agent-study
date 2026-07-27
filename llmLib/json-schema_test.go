package llmlib

import (
	"testing"
)

// 搞清楚 generate 的用法
func TestGenerate(t *testing.T) {
	type GetWeatherArgs struct {
		City string `json:"city" desc:"城市名"`
		Days int    `json:"days,omitempty" desc:"预报天数，默认 1"`
	}

	s := Generate(GetWeatherArgs{})

	t.Logf("结果：%v", *s)
	t.Log("通过测试")
}
