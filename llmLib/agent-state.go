// 文件职责：
// - 实现 Agent 状态机抽象，定义四个阶段：Thinking、Acting、Done、Error。
// - 定义 State 结构体作为一次 Agent 运行的完整快照，支持 JSON 序列化用于持久化。
// - 实现 Store 接口和 FileStore，支持会话状态的保存和恢复。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Phase 表示 Agent 的运行阶段，是状态机的核心状态。
type Phase string

const (
	PhaseThinking Phase = "thinking" // 调用模型，等待它决定下一步
	PhaseActing   Phase = "acting"   // 模型决定要用工具，代码执行工具
	PhaseDone     Phase = "done"     // 模型给出最终答案，正常结束
	PhaseError    Phase = "error"    // 遇到不可恢复错误，异常结束
)

// State 是一次 Agent 运行的完整快照，刻意设计成可 JSON 序列化。
// Agent 的"记忆"就是 Messages 列表，模型之所以能"记得"前几轮做了什么，
// 是因为每轮调用时都把完整历史重新发给它。
type State struct {
	Goal         string            `json:"goal"`     // 用户给的目标
	Messages     []Message         `json:"messages"` // 完整对话历史，含工具结果——这是 Agent 的"记忆"
	Step         int               `json:"step"`     // 已执行的步数
	Phase        Phase             `json:"phase"`    // 当前运行阶段
	Answer       string            `json:"answer,omitempty"`        // 终态时的最终答案
	Usage        Usage             `json:"usage"`                   // 累计 token 用量
	ActionCounts map[string]int    `json:"action_counts,omitempty"` // 重复动作检测，防止死循环
	StartedAt    time.Time         `json:"started_at"`              // 本轮开始时间
	UpdatedAt    time.Time         `json:"updated_at"`              // 最近一次状态更新时间
	Metadata     map[string]string `json:"metadata,omitempty"`      // 预留给业务侧扩展
	GoalAdded    bool              `json:"goal_added,omitempty"`    // 目标是否已添加到消息中
}

// Store 接口定义状态持久化的基本操作，可实现为文件存储、数据库存储等。
type Store interface {
	Save(ctx context.Context, sessionID string, st *State) error
	Load(ctx context.Context, sessionID string) (*State, error)
}

// FileStore 是基于文件系统的状态持久化实现，每个会话对应一个 JSON 文件。
type FileStore struct{ dir string }

// NewFileStore 创建一个新的 FileStore，dir 为存储目录。
func NewFileStore(dir string) *FileStore { return &FileStore{dir: dir} }

// Save 保存状态到文件，会话 ID 作为文件名。
func (store *FileStore) Save(_ context.Context, sessionID string, state *State) error {
	if store == nil {
		return fmt.Errorf("FileStore 未初始化")
	}
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(store.path(sessionID), raw, 0o600)
}

// Load 从文件加载状态，会话 ID 作为文件名。
func (store *FileStore) Load(_ context.Context, sessionID string) (*State, error) {
	if store == nil {
		return nil, fmt.Errorf("FileStore 未初始化")
	}
	raw, err := os.ReadFile(store.path(sessionID))
	if err != nil {
		return nil, fmt.Errorf("加载会话 %s 失败: %w", sessionID, err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.ActionCounts == nil {
		state.ActionCounts = make(map[string]int)
	}
	return &state, nil
}

// path 生成会话文件路径，确保文件名安全。
func (store *FileStore) path(sessionID string) string {
	name := filepath.Base(sessionID)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "default"
	}
	return filepath.Join(store.dir, name+".json")
}

// checkpoint 保存状态检查点，先清理空消息，再保存到内存和持久化存储。
func (agent *Agent) checkpoint(ctx context.Context, state *State) {
	state.Messages = dropEmptyAssistantMessages(state.Messages)
	agent.memory = state
	if agent.store == nil || agent.sessionID == "" {
		return
	}
	_ = agent.store.Save(ctx, agent.sessionID, state)
}

// dropEmptyAssistantMessages 移除空的助手消息，减少上下文长度。
func dropEmptyAssistantMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}
	out := messages[:0]
	for _, message := range messages {
		if message.Role == Assistant &&
			strings.TrimSpace(message.Content) == "" &&
			len(message.ToolCalls) == 0 {
			continue
		}
		out = append(out, message)
	}
	return out
}