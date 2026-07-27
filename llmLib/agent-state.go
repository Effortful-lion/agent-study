// 文件职责：
// - 定义 Agent 状态机：四个阶段 Thinking/Acting/Done/Error。
// - State 结构体是一次 Agent 运行的完整快照，支持 JSON 序列化用于持久化。
// - Store 接口和 FileStore 实现会话状态的保存和恢复。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/lg"
)

// Phase 表示 Agent 的运行阶段。
// 状态转换图：
//
//	     ┌──────────┐
//	     │ thinking │ ←──────────┐
//	     └────┬─────┘            │
//	有工具调用 │  无工具调用       │
//	          ↓                  │
//	     ┌──────────┐            │
//	     │ acting   │ ──────────┘
//	     └────┬─────┘  (执行完工具后)
//	      出错  │
//	          ↓
//	     ┌──────────┐
//	     │  error   │
//	     └──────────┘
//
//	     ┌──────────┐
//	     │   done   │  (最终答案)
//	     └──────────┘
type Phase string

const (
	PhaseThinking Phase = "thinking" // 调用模型，等待决策
	PhaseActing   Phase = "acting"   // 执行工具调用
	PhaseDone     Phase = "done"     // 模型给出最终答案
	PhaseError    Phase = "error"    // 不可恢复错误
)

// State 是一次 Agent 运行的完整快照，可 JSON 序列化。
// 核心思想：Agent 的"记忆"就是 Messages 列表。
type State struct {
	Goal         string            `json:"goal"`                    // 用户目标
	Messages     []Message         `json:"messages"`                // 完整对话历史
	Step         int               `json:"step"`                    // 已执行步数
	Phase        Phase             `json:"phase"`                   // 当前阶段
	Answer       string            `json:"answer,omitempty"`        // 最终答案
	Usage        Usage             `json:"usage"`                   // 累计 token 用量
	ActionCounts map[string]int    `json:"action_counts,omitempty"` // 重复动作检测
	StartedAt    time.Time         `json:"started_at"`              // 开始时间
	UpdatedAt    time.Time         `json:"updated_at"`              // 最近更新时间
	Metadata     map[string]string `json:"metadata,omitempty"`      // 业务侧扩展 + 内部暂存
	GoalAdded    bool              `json:"goal_added,omitempty"`    // 目标是否已添加到消息
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

// checkpoint 保存状态检查点。
func (agent *Agent) checkpoint(ctx context.Context, state *State) {
	state.Messages = dropEmptyAssistantMessages(state.Messages)
	agent.memory = state
	if agent.store == nil || agent.sessionID == "" {
		return
	}
	if err := agent.store.Save(ctx, agent.sessionID, state); err != nil {
		lg.Frame.Error("checkpoint: 保存状态失败", lg.Fields{"error": err, "session": agent.sessionID})
	}
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
