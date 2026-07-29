package lg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"warning", LevelWarn, false},
		{"error", LevelError, false},
		{"FATAL", LevelFatal, false},
		{"unknown", LevelInfo, true},
	}

	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if err == nil && got != tt.expected {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestEntryFormat(t *testing.T) {
	entry := &Entry{
		Level:   LevelInfo,
		Module:  "user",
		File:    "service.go:42",
		Message: "用户登录成功",
		Fields:  Fields{"uid": 123, "ip": "10.0.0.1"},
	}
	formatted := entry.Format()
	if !strings.Contains(formatted, "INFO") {
		t.Error("missing level")
	}
	if !strings.Contains(formatted, "[user]") {
		t.Error("missing module")
	}
	if !strings.Contains(formatted, "service.go:42") {
		t.Error("missing file location")
	}
	if !strings.Contains(formatted, "用户登录成功") {
		t.Error("missing message")
	}
	if !strings.Contains(formatted, "uid=123") {
		t.Error("missing uid field")
	}
}

func TestConsoleWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)

	err := w.Write(&Entry{Level: LevelInfo, Message: "hello"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", output)
	}
}

func TestConsoleWriterLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelWarn)

	_ = w.Write(&Entry{Level: LevelDebug, Message: "debug"})
	if buf.Len() > 0 {
		t.Error("debug message should be filtered")
	}

	_ = w.Write(&Entry{Level: LevelError, Message: "error"})
	if !strings.Contains(buf.String(), "error") {
		t.Error("error message should be written")
	}
}

func TestFileWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewFileWriter(path, LevelInfo)
	if err != nil {
		t.Fatalf("NewFileWriter failed: %v", err)
	}
	defer w.Close()

	_ = w.Write(&Entry{Level: LevelInfo, Message: "file log test"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if !strings.Contains(string(data), "file log test") {
		t.Errorf("expected log in file, got: %s", string(data))
	}
}

func TestFileWriterAutoCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "logs")
	path := filepath.Join(dir, "app.log")

	w, err := NewFileWriter(path, LevelInfo)
	if err != nil {
		t.Fatalf("NewFileWriter should auto-create dir: %v", err)
	}
	defer w.Close()
	defer os.RemoveAll(dir)

	_ = w.Write(&Entry{Level: LevelInfo, Message: "auto dir test"})
}

func TestMultiWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	w1 := NewConsoleWriter(&buf1, LevelInfo)
	w2 := NewConsoleWriter(&buf2, LevelDebug)
	mw := NewMultiWriter(w1, w2)

	_ = mw.Write(&Entry{Level: LevelInfo, Message: "shared"})

	if !strings.Contains(buf1.String(), "shared") {
		t.Error("buf1 should contain log")
	}
	if !strings.Contains(buf2.String(), "shared") {
		t.Error("buf2 should contain log")
	}
}

func TestRouter(t *testing.T) {
	var defaultBuf, userBuf, shopBuf bytes.Buffer

	router := NewRouter(NewConsoleWriter(&defaultBuf, LevelInfo))
	router.Route("user", NewConsoleWriter(&userBuf, LevelDebug))
	router.Route("shop", NewConsoleWriter(&shopBuf, LevelInfo))

	logger := New(router)

	logger.Module("user").Info("用户登录")
	if !strings.Contains(userBuf.String(), "用户登录") {
		t.Error("user log should go to userBuf")
	}
	if defaultBuf.Len() > 0 {
		t.Error("user log should NOT go to defaultBuf")
	}

	logger.Module("shop").Warn("库存不足")
	if !strings.Contains(shopBuf.String(), "库存不足") {
		t.Error("shop log should go to shopBuf")
	}

	logger.Module("unknown").Error("未知错误")
	if !strings.Contains(defaultBuf.String(), "未知错误") {
		t.Error("unknown module should go to defaultBuf")
	}
}

func TestRouterUnroute(t *testing.T) {
	var defaultBuf, userBuf bytes.Buffer
	router := NewRouter(NewConsoleWriter(&defaultBuf, LevelInfo))
	router.Route("user", NewConsoleWriter(&userBuf, LevelInfo))

	logger := New(router)

	logger.Module("user").Info("before unroute")
	if userBuf.Len() == 0 {
		t.Error("should go to userBuf before unroute")
	}

	router.Unroute("user")

	logger.Module("user").Info("after unroute")
	if !strings.Contains(defaultBuf.String(), "after unroute") {
		t.Error("after unroute, should go to defaultBuf")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w).With(Fields{"app": "myapp", "env": "test"})

	logger.Info("服务启动", Fields{"port": 8080})

	output := buf.String()
	if !strings.Contains(output, "app=myapp") {
		t.Error("missing fixed field 'app'")
	}
	if !strings.Contains(output, "env=test") {
		t.Error("missing fixed field 'env'")
	}
	if !strings.Contains(output, "port=8080") {
		t.Error("missing per-log field 'port'")
	}
}

func TestLoggerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelWarn)
	logger := New(w)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")

	output := buf.String()
	if strings.Contains(output, "debug") || strings.Contains(output, "info") {
		t.Error("debug/info should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn should be output")
	}
}

func TestLoggerModuleWithInherit(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w).With(Fields{"service": "api"})

	userLog := logger.Module("user")
	userLog.Info("登录", Fields{"uid": 1})

	output := buf.String()
	if !strings.Contains(output, "[user]") {
		t.Error("missing module prefix")
	}
	if !strings.Contains(output, "service=api") {
		t.Error("missing inherited field")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	SetDefault(New(NewConsoleWriter(&buf, LevelDebug)))

	Info("包级别日志", Fields{"key": "val"})
	output := buf.String()
	if !strings.Contains(output, "包级别日志") {
		t.Error("missing message")
	}
	if !strings.Contains(output, "key=val") {
		t.Error("missing field")
	}
}

func TestFormatFunctions(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w)

	logger.Infof("用户 %d 登录, 来自 %s", 123, "10.0.0.1")
	output := buf.String()
	if !strings.Contains(output, "用户 123 登录, 来自 10.0.0.1") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestCallerLocation(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelDebug)
	logger := New(w)

	logger.Debug("caller test")
	output := buf.String()
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("missing caller file, got: %s", output)
	}
}

func TestModuleLogger_DefaultDisabled(t *testing.T) {
	m := Module("test_default_disabled")
	if m.Enabled() {
		t.Error("module should be disabled by default")
	}
	if m.BufferSize() != 0 {
		t.Error("buffer should be empty when disabled")
	}
}

func TestModuleLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_info")
	m.writer = NewConsoleWriter(&buf, LevelDebug)

	m.Info("hello from module")
	if !strings.Contains(buf.String(), "hello from module") {
		t.Error("log should appear on console")
	}
	if !strings.Contains(buf.String(), "[test_info]") {
		t.Error("log should include module name")
	}
}

func TestModuleLogger_BufferingWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_buffering")
	m.writer = NewConsoleWriter(&buf, LevelDebug)

	m.Enable()
	m.Info("buffered message")

	if m.BufferSize() != 1 {
		t.Errorf("buffer should have 1 entry, got %d", m.BufferSize())
	}

	m.Disable()
	m.Info("not buffered")
	if m.BufferSize() != 1 {
		t.Errorf("buffer should still have 1 entry, got %d", m.BufferSize())
	}
}

func TestModuleLogger_WriteNow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "module_test.log")

	m := Module("test_writenow")
	m.writer = NewConsoleWriter(os.Stdout, LevelDebug)

	m.SetPath(path)
	m.Info("first write")
	m.Info("second write")

	if err := m.WriteNow(); err != nil {
		t.Fatalf("WriteNow failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "first write") {
		t.Error("file should contain first write")
	}
	if !strings.Contains(content, "second write") {
		t.Error("file should contain second write")
	}

	if m.BufferSize() != 0 {
		t.Error("buffer should be empty after WriteNow")
	}
}

func TestModuleLogger_SetPath(t *testing.T) {
	m := Module("test_setpath")
	defaultPath := m.Path()
	if defaultPath != "./test_setpath.log" {
		t.Errorf("expected default path ./test_setpath.log, got %s", defaultPath)
	}

	m.SetPath("/absolute/path/custom.log")
	if m.Path() != "/absolute/path/custom.log" {
		t.Errorf("path not updated: %s", m.Path())
	}
	if !m.Enabled() {
		t.Error("SetPath should implicitly enable the module")
	}
}

func TestModuleLogger_AutoCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	path := filepath.Join(dir, "deep.log")

	m := Module("test_autocreate")
	m.SetPath(path)
	m.Info("deep directory test")

	if err := m.WriteNow(); err != nil {
		t.Fatalf("WriteNow should auto-create dir: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file should exist after WriteNow")
	}
}

func TestModuleLogger_ClearBuffer(t *testing.T) {
	m := Module("test_clear")
	m.Enable()
	m.Info("to be cleared")
	if m.BufferSize() != 1 {
		t.Errorf("expected 1 entry, got %d", m.BufferSize())
	}

	m.ClearBuffer()
	if m.BufferSize() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", m.BufferSize())
	}
}

func TestModuleLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_levels")
	m.writer = NewConsoleWriter(&buf, LevelDebug)

	m.Debug("debug msg")
	if !strings.Contains(buf.String(), "debug msg") {
		t.Error("debug should pass when level=Debug")
	}

	buf.Reset()
	m.Warn("warn msg")
	if !strings.Contains(buf.String(), "warn msg") {
		t.Error("warn should pass")
	}

	buf.Reset()
	m.Error("error msg")
	if !strings.Contains(buf.String(), "error msg") {
		t.Error("error should pass")
	}
}

func TestModuleLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_levelfilter")
	m.writer = NewConsoleWriter(&buf, LevelWarn)

	m.Debug("debug")
	m.Info("info")
	m.Warn("warn")
	m.Error("error")

	output := buf.String()
	if strings.Contains(output, "debug") || strings.Contains(output, "info") {
		t.Error("debug/info should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn should pass")
	}
	if !strings.Contains(output, "error") {
		t.Error("error should pass")
	}
}

func TestModuleLogger_Singleton(t *testing.T) {
	m1 := Module("test_singleton")
	m2 := Module("test_singleton")

	m1.SetPath("/tmp/singleton.log")
	if m2.Path() != "/tmp/singleton.log" {
		t.Error("modules with same name should be the same instance")
	}
}

func TestModuleLogger_With(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_with")
	m.writer = NewConsoleWriter(&buf, LevelInfo)

	sub := m.With(Fields{"request_id": "abc123"})
	sub.Info("sub module log")

	output := buf.String()
	if !strings.Contains(output, "request_id=abc123") {
		t.Error("should contain fixed field")
	}
}

func TestModuleLogger_EmptyBufferWriteNow(t *testing.T) {
	m := Module("test_empty_write")
	err := m.WriteNow()
	if err != nil {
		t.Errorf("WriteNow on empty buffer should not error: %v", err)
	}
}

func TestModuleLogger_FormatFunctions(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_format")
	m.writer = NewConsoleWriter(&buf, LevelInfo)

	m.Infof("user %d logged in from %s", 42, "10.0.0.1")
	output := buf.String()
	if !strings.Contains(output, "user 42 logged in from 10.0.0.1") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestModuleLogger_CallerLocation(t *testing.T) {
	var buf bytes.Buffer
	m := Module("test_caller")
	m.writer = NewConsoleWriter(&buf, LevelDebug)

	m.Debug("caller location test")
	output := buf.String()
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("missing caller file, got: %s", output)
	}
}
