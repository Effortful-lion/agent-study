# KP09 — 日志系统（lg 包）

## 标题和概述

`lg` 包是 llmLib 的 **结构化日志系统**，提供了一套完整的日志记录解决方案。它负责：

- `Logger` 面向用户的日志记录器，支持模块化日志和固定字段注入
- `Entry` 封装单条日志的完整上下文信息（时间、级别、模块、调用位置、消息、结构化字段）
- `Level` 日志级别体系（Debug/Info/Warn/Error/Fatal），支持字符串解析
- `Writer` 日志输出接口，可扩展到任意目标（控制台、文件、网络、消息队列等）
- `Router` 按模块名将日志分流到不同的 Writer，实现模块化日志管理
- 内置 `Frame` 框架日志器，用于 llmLib 库自身的运行时错误和警告

## 核心概念

### 1. Logger — 日志记录器

```go
type Logger struct {
    module     string  // 所属模块
    writer     Writer  // 输出目标（通常是 Router）
    fields     Fields  // 预置的固定字段
    callerSkip int     // 调用栈跳过层数
}
```

`Logger` 是面向用户的核心类型，支持以下使用模式：
- **直接使用**：`lg.Info("服务启动", lg.Fields{"port": 8080})`
- **模块化使用**：`lg.Module("user").Info("用户登录", lg.Fields{"uid": 123})`
- **实例化使用**：`userLog := lg.New(writer).Module("user")`

### 2. Entry — 日志记录

```go
type Entry struct {
    Time    time.Time  // 日志时间
    Level   Level      // 日志级别
    Module  string     // 模块/子系统名称
    File    string     // 调用位置，如 "service.go:42"
    Message string     // 日志内容
    Fields  Fields     // 结构化字段
}
```

`Entry` 是日志系统的最小单元，`Format()` 方法提供默认的文本格式化输出。

### 3. Level — 日志级别

```go
type Level int

const (
    LevelDebug Level = iota  // 0 - 调试
    LevelInfo                // 1 - 信息
    LevelWarn                // 2 - 警告
    LevelError                // 3 - 错误
    LevelFatal               // 4 - 致命
)
```

数值越小越严重。支持 `ParseLevel(s)` 从字符串解析（不区分大小写）。

### 4. Writer — 输出接口

```go
type Writer interface {
    Write(entry *Entry) error   // 写入一条日志
    Level() Level               // 返回接受的最低日志级别
    Close() error               // 关闭 Writer，释放资源
}
```

三种内置实现：

| 实现 | 说明 |
|---|---|
| `ConsoleWriter` | 写入 `io.Writer`（通常为 `os.Stdout`/`os.Stderr`），带互斥锁保护 |
| `FileWriter` | 写入文件，支持自动创建目录，追加模式 |
| `MultiWriter` | 同时写入多个 Writer，取最低级别 |

### 5. Router — 日志路由器

```go
type Router struct {
    defaultWriter Writer              // 未匹配模块时的默认输出
    routes        map[string]Writer  // 模块名 → Writer 的路由表
    mu            sync.RWMutex       // 读写锁
}
```

`Router` 实现了 `Writer` 接口，按 `Entry.Module` 将日志分流到不同的输出目标。

### 6. 框架内置日志器

```go
var Frame = New(NewConsoleWriter(os.Stderr, LevelWarn)).Module("frame")
```

`Frame` 是 llmLib 库内部专用日志器，模块名为 `"frame"`，默认输出到 `os.Stderr`，级别为 `LevelWarn`。可通过 `SetFrameWriter(w)` 替换输出目标。

### 7. Fields — 结构化字段

```go
type Fields map[string]any
```

用于携带业务上下文，会自动附加到每条日志中并格式化输出。

## 类型/函数清单

### Logger 核心类型与方法
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Logger` | `lg/logger.go` | 日志记录器结构体 |
| `New(writer)` | `lg/logger.go` | 创建日志记录器 |
| `(*Logger).Module(module)` | `lg/logger.go` | 创建模块子 Logger |
| `(*Logger).With(fields)` | `lg/logger.go` | 创建带固定字段的子 Logger |
| `(*Logger).Debug(msg, fields)` | `lg/logger.go` | 输出 Debug 日志 |
| `(*Logger).Info(msg, fields)` | `lg/logger.go` | 输出 Info 日志 |
| `(*Logger).Warn(msg, fields)` | `lg/logger.go` | 输出 Warn 日志 |
| `(*Logger).Error(msg, fields)` | `lg/logger.go` | 输出 Error 日志 |
| `(*Logger).Fatal(msg, fields)` | `lg/logger.go` | 输出 Fatal 日志并退出 |
| `(*Logger).Debugf(format, args)` | `lg/logger.go` | 格式化 Debug 日志 |
| `(*Logger).Infof(format, args)` | `lg/logger.go` | 格式化 Info 日志 |
| `(*Logger).Warnf(format, args)` | `lg/logger.go` | 格式化 Warn 日志 |
| `(*Logger).Errorf(format, args)` | `lg/logger.go` | 格式化 Error 日志 |
| `(*Logger).Fatalf(format, args)` | `lg/logger.go` | 格式化 Fatal 日志并退出 |

### Entry 与 Fields
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Entry` | `lg/entry.go` | 日志记录结构体 |
| `Fields` | `lg/entry.go` | 结构化字段类型（`map[string]any`） |
| `(*Entry).Format()` | `lg/entry.go` | 格式化日志为字符串 |
| `(Fields).format()` | `lg/entry.go` | 格式化字段为键值对（内部） |

### Level 日志级别
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Level` | `lg/level.go` | 日志级别类型（`int`） |
| `LevelDebug` | `lg/level.go` | Debug 级别常量 |
| `LevelInfo` | `lg/level.go` | Info 级别常量 |
| `LevelWarn` | `lg/level.go` | Warn 级别常量 |
| `LevelError` | `lg/level.go` | Error 级别常量 |
| `LevelFatal` | `lg/level.go` | Fatal 级别常量 |
| `(Level).String()` | `lg/level.go` | 级别转字符串 |
| `ParseLevel(s)` | `lg/level.go` | 字符串转级别（不区分大小写） |

### Writer 接口及实现
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Writer` | `lg/writer.go` | 日志输出接口 |
| `ConsoleWriter` | `lg/writer.go` | 控制台输出实现 |
| `NewConsoleWriter(out, level)` | `lg/writer.go` | 创建控制台 Writer |
| `FileWriter` | `lg/writer.go` | 文件输出实现 |
| `NewFileWriter(path, level)` | `lg/writer.go` | 创建文件 Writer（自动建目录） |
| `MultiWriter` | `lg/writer.go` | 多路输出实现 |
| `NewMultiWriter(writers...)` | `lg/writer.go` | 创建多路 Writer |

### Router 日志路由
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Router` | `lg/router.go` | 日志路由器（实现 Writer 接口） |
| `NewRouter(defaultWriter)` | `lg/router.go` | 创建路由器 |
| `(*Router).Route(module, writer)` | `lg/router.go` | 注册模块路由 |
| `(*Router).Unroute(module)` | `lg/router.go` | 取消模块路由 |
| `(*Router).Resolve(module)` | `lg/router.go` | 查找模块对应的 Writer |
| `(*Router).Routes()` | `lg/router.go` | 获取所有已注册路由的模块名 |
| `(*Router).Level()` | `lg/router.go` | 获取所有路由中的最低级别 |
| `(*Router).Write(entry)` | `lg/router.go` | 按模块路由写入日志 |
| `(*Router).Close()` | `lg/router.go` | 关闭路由器及所有 Writer |

### 包级别便捷函数
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `defaultLogger` | `lg/logger.go` | 默认 Logger 实例 |
| `SetDefault(l)` | `lg/logger.go` | 替换默认 Logger |
| `Default()` | `lg/logger.go` | 获取默认 Logger |
| `Module(module)` | `lg/logger.go` | 使用默认 Logger 创建子 Logger |
| `Frame` | `lg/logger.go` | 框架内置日志器（stderr, Warn 级别） |
| `SetFrameWriter(w)` | `lg/logger.go` | 替换 Frame 日志器输出 |
| `Debug/Info/Warn/Error/Fatal` | `lg/logger.go` | 包级别便捷日志函数 |
| `Debugf/Infof/Warnf/Errorf/Fatalf` | `lg/logger.go` | 包级别便捷格式化日志函数 |

## 使用示例

### 基础使用

```go
import (
    "github.com/Effortful-lion/agent-study/llmLib/lg"
)

lg.Info("服务启动", lg.Fields{"port": 8080})
lg.Error("连接失败", lg.Fields{"db": "mysql", "attempt": 3})
lg.Debugf("处理请求: %s", req.ID)
```

### 模块化日志

```go
userLog := lg.Module("user")
userLog.Info("用户登录", lg.Fields{"uid": 123, "username": "alice"})
userLog.Warn("密码即将过期", lg.Fields{"uid": 123})

shopLog := lg.Module("shop")
shopLog.Error("库存不足", lg.Fields{"sku": "A001", "stock": 0})
```

### 使用 Router 实现模块化分流

```go
import (
    "os"
    "github.com/Effortful-lion/agent-study/llmLib/lg"
)

// 1. 创建各模块的文件 Writer
userWriter, _ := lg.NewFileWriter("./logs/user.log", lg.LevelInfo)
shopWriter, _ := lg.NewFileWriter("./logs/shop.log", lg.LevelInfo)
defaultWriter := lg.NewConsoleWriter(os.Stdout, lg.LevelInfo)

// 2. 创建路由器并注册路由
router := lg.NewRouter(defaultWriter)
router.Route("user", userWriter)
router.Route("shop", shopWriter)

// 3. 创建 Logger
logger := lg.New(router)

// 4. 日志自动分流
logger.Module("user").Info("用户登录")   // → ./logs/user.log
logger.Module("shop").Warn("库存不足")   // → ./logs/shop.log
logger.Module("other").Debug("其他")     // → 控制台
```

### 使用 MultiWriter 同时输出

```go
consoleWriter := lg.NewConsoleWriter(os.Stdout, lg.LevelInfo)
fileWriter, _ := lg.NewFileWriter("./logs/app.log", lg.LevelWarn)

multi := lg.NewMultiWriter(consoleWriter, fileWriter)
logger := lg.New(multi)

logger.Info("这条日志同时输出到控制台和文件")
```

### 替换 Frame 日志器输出

```go
fileWriter, _ := lg.NewFileWriter("./logs/llmlib.log", lg.LevelWarn)
lg.SetFrameWriter(fileWriter)
```

### 自定义 Writer 实现

```go
type NetworkWriter struct {
    endpoint string
    level    lg.Level
}

func (w *NetworkWriter) Write(entry *lg.Entry) error {
    // 通过 UDP/TCP 发送到远程日志服务
    data := []byte(entry.Format())
    _, err := net.Dial("udp", w.endpoint)
    return err
}

func (w *NetworkWriter) Level() Level { return w.level }
func (w *NetworkWriter) Close() error  { return nil }
```

### 替换默认 Logger

```go
router := lg.NewRouter(lg.NewConsoleWriter(os.Stdout, lg.LevelInfo))
lg.SetDefault(lg.New(router))

lg.Info("现在使用自定义默认 Logger")
```

## 关联知识点

- **KP05-Agent运行时**：Agent 运行时的日志输出通过 `lg` 包实现，可自定义日志级别和输出目标
- **KP10-信号处理**：信号处理中的优雅关闭流程通常配合日志系统记录 shutdown 事件
- **KP11-状态管理**：状态机执行过程中的状态转换、错误等信息可通过 `lg` 包记录
- **KP08-命令行参数**：可通过命令行参数配置日志级别（如 `-log-level debug`）和日志输出目标