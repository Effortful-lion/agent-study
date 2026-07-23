package llmlib

import (
	"flag"
	"fmt"
)

// CommandBuilder 封装一层可扩展的命令行参数注册和解析能力。
type CommandBuilder struct {
	fs     *flag.FlagSet
	values map[string]*string
}

// NewCommandBuilder 创建一个可用于插件式扩展参数的构建器。
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{
		fs:     flag.NewFlagSet("llmlib", flag.ContinueOnError),
		values: make(map[string]*string),
	}
}

// Register 注册一个 string 类型参数，重复注册同名参数时返回错误。
func (b *CommandBuilder) Register(name, usage, defaultValue string) error {
	if _, exists := b.values[name]; exists {
		return fmt.Errorf("command flag already registered: %s", name)
	}

	value := new(string)
	*value = defaultValue
	b.fs.StringVar(value, name, defaultValue, usage)
	b.values[name] = value
	return nil
}

// Parse 解析传入参数，并返回未被 flag 消费的剩余位置参数。
func (b *CommandBuilder) Parse(args []string) ([]string, error) {
	if err := b.fs.Parse(args); err != nil {
		return nil, err
	}
	return b.fs.Args(), nil
}

// Get 返回指定参数当前解析后的值。
func (b *CommandBuilder) Get(name string) string {
	value, ok := b.values[name]
	if !ok || value == nil {
		return ""
	}
	return *value
}

// LoadCommands 创建一个已注册常用命令行参数的构建器。
func LoadCommands() *CommandBuilder {
	builder := NewCommandBuilder()
	_ = builder.Register("question", "提问内容", "")
	return builder
}

// MustRegister 在注册失败时直接 panic，适合初始化阶段快速挂载常用参数。
func (b *CommandBuilder) MustRegister(name, usage, defaultValue string) {
	if err := b.Register(name, usage, defaultValue); err != nil {
		panic(err)
	}
}
