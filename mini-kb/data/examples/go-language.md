# Go 语言教程

## 什么是 Go？

Go（也称为 Golang）是由 Google 开发的开源编程语言，于 2009 年首次发布。
Go 语言的设计目标是简洁、高效、可靠，特别适合构建并发和网络服务。

## Go 的核心特性

1. **简洁的语法**：Go 的语法非常简洁，学习曲线平缓。关键字只有 25 个，远少于 C++ 或 Java。
2. **编译速度快**：Go 编译成原生机器码，启动速度极快，适合微服务架构。
3. **原生并发**：Goroutine 是 Go 的轻量级线程，由 Go 运行时调度。Channel 是 Goroutine 之间通信的管道。
4. **垃圾回收**：Go 自带垃圾回收机制，开发者无需手动管理内存。
5. **静态类型**：Go 是静态类型语言，编译时进行类型检查，减少运行时错误。
6. **标准库丰富**：Go 标准库包含 HTTP 服务器、JSON 处理、加密等常用功能。

## Goroutine 和 Channel

Goroutine 是 Go 的并发执行单元，由 `go` 关键字启动：

```go
go func() {
    fmt.Println("Hello from goroutine")
}()
```

Channel 用于 Goroutine 之间的通信：

```go
ch := make(chan int)
go func() {
    ch <- 42 // 发送值
}()
val := <-ch // 接收值
```

## 接口

Go 的接口是隐式实现的，不需要显式声明：

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type MyReader struct{}

func (m MyReader) Read(p []byte) (int, error) {
    // 实现...
}
```

## 错误处理

Go 使用多值返回处理错误：

```go
data, err := os.ReadFile("file.txt")
if err != nil {
    log.Fatal(err)
}
```

## 总结

Go 语言以其简洁的设计和强大的并发能力，在现代云原生开发中占据了重要地位。
Docker、Kubernetes、etcd 等知名项目都是用 Go 编写的。
