package main

import (
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib"
)

func main() {
	err := llmlib.LoadDotEnv()
	if err != nil {
		fmt.Printf("加载 .env 文件失败: %v\n", err)
		return
	}
	fmt.Println(".env 文件加载成功")

	services, err := llmlib.LoadAll()
	if err != nil {
		fmt.Printf("加载服务配置失败: %v\n", err)
		return
	}

	fmt.Printf("已加载 %d 个服务:\n", len(services))
	for i, service := range services {
		fmt.Printf("%d. Provider: %s, Model: %s\n", i+1, service.Provider.Name(), service.Config.Model)
	}
}