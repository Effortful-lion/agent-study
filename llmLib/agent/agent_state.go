// 文件职责：
// - 定义 Agent 状态机：四个阶段 Thinking/Acting/Done/Error。
// - State 结构体是一次 Agent 运行的完整快照，支持 JSON 序列化用于持久化。
// - Store 接口和 FileStore 实现会话状态的保存和恢复。

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/lg"
)

// Phase 表示 Agent 的运行阶段。
type Phase string

const (
	PhaseThinking Phase = "thinking"
	PhaseActing   Phase = "acting"
	PhaseDone     Phase = "done"
	PhaseError    Phase = "error"
)

// State 是一次 Agent 运行的完整快照，可 JSON 序列化。
type State struct {
	Goal         string            `json:"goal"`
	Messages     []core.Message    `json:"messages"`
	Step         int               `json:"step"`
	Phase        Phase             `json:"phase"`
	Answer       string            `json:"answer,omitempty"`
	Usage        core.Usage        `json:"usage"`
	ActionCounts map[string]int    `json:"action_counts,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	GoalAdded    bool              `json:"goal_added,omitempty"`
}

// Store 定义状态持久化接口。
type Store interface {
	Save(ctx context.Context, sessionID string, st *State) error
	Load(ctx context.Context, sessionID string) (*State, error)
}

// FileStore 是基于文件系统的状态持久化实现。
type FileStore struct{ dir string }

func NewFileStore(dir string) *FileStore { return &FileStore{dir: dir} }

func (store *FileStore) Save(_ context.Context, sessionID string, state *State) error {
	if store == nil {
		return fmt.Errorf("FileStore 未初始化")
	}
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		lg.Frame.Error("FileStore: 创建目录失败", lg.Fields{"dir": store.dir, "error": err})
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		lg.Frame.Error("FileStore: 序列化失败", lg.Fields{"session": sessionID, "error": err})
		return err
	}
	return os.WriteFile(store.path(sessionID), raw, 0o600)
}

func (store *FileStore) Load(_ context.Context, sessionID string) (*State, error) {
	if store == nil {
		return nil, fmt.Errorf("FileStore 未初始化")
	}
	raw, err := os.ReadFile(store.path(sessionID))
	if err != nil {
		lg.Frame.Error("FileStore: 读取失败", lg.Fields{"session": sessionID, "error": err})
		return nil, fmt.Errorf("加载会话 %s 失败: %w", sessionID, err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		lg.Frame.Error("FileStore: 反序列化失败", lg.Fields{"session": sessionID, "error": err})
		return nil, err
	}
	if state.ActionCounts == nil {
		state.ActionCounts = make(map[string]int)
	}
	return &state, nil
}

func (store *FileStore) path(sessionID string) string {
	name := filepath.Base(sessionID)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "default"
	}
	return filepath.Join(store.dir, name+".json")
}

// dropEmptyAssistantMessages 移除空的助手消息，减少上下文长度。
func dropEmptyAssistantMessages(messages []core.Message) []core.Message {
	if len(messages) == 0 {
		return messages
	}
	out := messages[:0]
	for _, message := range messages {
		if message.Role == core.Assistant &&
			strings.TrimSpace(message.Content) == "" &&
			len(message.ToolCalls) == 0 {
			continue
		}
		out = append(out, message)
	}
	return out
}
