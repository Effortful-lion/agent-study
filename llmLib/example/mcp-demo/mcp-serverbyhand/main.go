// 文件职责：
// - 主程序入口
// - 选择要运行的演示模式

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// 解析命令行参数
	mode := flag.String("mode", "full", "演示模式: full=完整演示, client=仅客户端, secure=仅安全演示")
	flag.Parse()

	switch *mode {
	case "full":
		fmt.Println("运行完整演示...")
		// 注意：由于 Go 的单包限制，实际需要拆分成多个可执行文件
		fmt.Println("请使用以下命令分别运行:")
		fmt.Println("  go run server.go              # 启动手写 Server")
		fmt.Println("  go run client.go              # 运行完整客户端演示")
		fmt.Println("  go run secure_demo.go         # 运行安全净化演示")
	case "client":
		fmt.Println("请运行: go run client.go")
	case "secure":
		fmt.Println("请运行: go run secure_demo.go")
	default:
		fmt.Printf("未知模式: %s\n", *mode)
		fmt.Println("可用模式: full, client, secure")
		os.Exit(1)
	}
}
