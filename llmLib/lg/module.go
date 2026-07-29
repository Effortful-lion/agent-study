package lg

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ModuleLogger struct {
	name       string
	enabled    bool
	path       string
	level      Level
	writer     Writer
	fields     Fields
	buffer     []*Entry
	callerSkip int
	mu         sync.Mutex
}

var (
	moduleMu sync.RWMutex
	modules  = make(map[string]*ModuleLogger)
)

func Module(name string) *ModuleLogger {
	moduleMu.RLock()
	if m, ok := modules[name]; ok {
		moduleMu.RUnlock()
		return m
	}
	moduleMu.RUnlock()

	moduleMu.Lock()
	defer moduleMu.Unlock()
	if m, ok := modules[name]; ok {
		return m
	}
	m := &ModuleLogger{
		name:       name,
		enabled:    false,
		path:       "./" + name + ".log",
		level:      LevelInfo,
		writer:     NewConsoleWriter(os.Stdout, LevelInfo),
		callerSkip: 2,
	}
	modules[name] = m
	return m
}

func (m *ModuleLogger) SetPath(path string) *ModuleLogger {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.path = path
	m.enabled = true
	return m
}

func (m *ModuleLogger) Enable() *ModuleLogger {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
	return m
}

func (m *ModuleLogger) Disable() *ModuleLogger {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
	return m
}

func (m *ModuleLogger) WriteNow() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.buffer) == 0 {
		return nil
	}

	dir := filepath.Dir(m.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("lg: create dir %s: %w", dir, err)
		}
	}

	f, err := os.OpenFile(m.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("lg: open file %s: %w", m.path, err)
	}
	defer f.Close()

	for _, entry := range m.buffer {
		fmt.Fprintln(f, entry.Format())
	}
	m.buffer = nil
	return nil
}

func (m *ModuleLogger) ClearBuffer() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer = nil
}

func (m *ModuleLogger) BufferSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buffer)
}

func (m *ModuleLogger) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *ModuleLogger) Path() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.path
}

func (m *ModuleLogger) With(fields Fields) *ModuleLogger {
	merged := Fields{}
	for k, v := range m.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &ModuleLogger{
		name:       m.name,
		enabled:    m.enabled,
		path:       m.path,
		level:      m.level,
		writer:     m.writer,
		fields:     merged,
		buffer:     m.buffer,
		callerSkip: m.callerSkip,
	}
}

func (m *ModuleLogger) Debug(msg string, fields ...Fields) {
	m.log(LevelDebug, msg, fields...)
}

func (m *ModuleLogger) Info(msg string, fields ...Fields) {
	m.log(LevelInfo, msg, fields...)
}

func (m *ModuleLogger) Warn(msg string, fields ...Fields) {
	m.log(LevelWarn, msg, fields...)
}

func (m *ModuleLogger) Error(msg string, fields ...Fields) {
	m.log(LevelError, msg, fields...)
}

func (m *ModuleLogger) Fatal(msg string, fields ...Fields) {
	m.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

func (m *ModuleLogger) Debugf(format string, args ...any) {
	m.log(LevelDebug, fmt.Sprintf(format, args...))
}

func (m *ModuleLogger) Infof(format string, args ...any) {
	m.log(LevelInfo, fmt.Sprintf(format, args...))
}

func (m *ModuleLogger) Warnf(format string, args ...any) {
	m.log(LevelWarn, fmt.Sprintf(format, args...))
}

func (m *ModuleLogger) Errorf(format string, args ...any) {
	m.log(LevelError, fmt.Sprintf(format, args...))
}

func (m *ModuleLogger) Fatalf(format string, args ...any) {
	m.log(LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (m *ModuleLogger) log(level Level, msg string, fields ...Fields) {
	if level < m.writer.Level() {
		return
	}

	merged := Fields{}
	for k, v := range m.fields {
		merged[k] = v
	}
	for _, f := range fields {
		for k, v := range f {
			merged[k] = v
		}
	}

	entry := &Entry{
		Time:    time.Now(),
		Level:   level,
		Module:  m.name,
		File:    caller(m.callerSkip),
		Message: msg,
		Fields:  merged,
	}

	_ = m.writer.Write(entry)

	m.mu.Lock()
	if m.enabled {
		m.buffer = append(m.buffer, entry)
	}
	m.mu.Unlock()
}
