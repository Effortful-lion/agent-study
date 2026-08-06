// Package memory 管理会话历史（短期记忆）和用户偏好（长期记忆）。
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionTurn 记录一轮对话。
type SessionTurn struct {
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Sources  []string `json:"sources"`
	Tools    []string `json:"tools,omitempty"`
	Time     string   `json:"time"`
}

// Session 是完整的会话记录。
type Session struct {
	ID        string        `json:"id"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
	Turns     []SessionTurn `json:"turns"`
}

// UserPreference 记录用户偏好。
type UserPreference struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Manager 管理会话记忆和用户偏好。
type Manager struct {
	dir      string
	sessions map[string]*Session
	prefs    map[string]UserPreference
	mu       sync.RWMutex
}

// NewManager 创建记忆管理器。
func NewManager(dir string) *Manager {
	os.MkdirAll(filepath.Join(dir, "sessions"), 0o700)
	return &Manager{
		dir:      dir,
		sessions: make(map[string]*Session),
		prefs:    make(map[string]UserPreference),
	}
}

// LoadSession 加载或创建会话。
func (m *Manager) LoadSession(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[id]; ok {
		return s
	}

	path := filepath.Join(m.dir, "sessions", id+".json")
	if data, err := os.ReadFile(path); err == nil {
		var s Session
		if err := json.Unmarshal(data, &s); err == nil {
			m.sessions[id] = &s
			return &s
		}
	}

	return &Session{
		ID:        id,
		CreatedAt: now(),
		UpdatedAt: now(),
		Turns:     []SessionTurn{},
	}
}

// SaveSession 持久化会话。
func (m *Manager) SaveSession(s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s.UpdatedAt = now()
	m.sessions[s.ID] = s

	path := filepath.Join(m.dir, "sessions", s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// AddTurn 添加一轮对话到会话。
func (m *Manager) AddTurn(s *Session, turn SessionTurn) {
	turn.Time = now()
	s.Turns = append(s.Turns, turn)
}

// History 返回会话的最近 N 轮对话。
func (s *Session) History(n int) []SessionTurn {
	if n <= 0 || n > len(s.Turns) {
		return s.Turns
	}
	return s.Turns[len(s.Turns)-n:]
}

// TurnCount 返回对话轮数。
func (s *Session) TurnCount() int {
	return len(s.Turns)
}

// AddPreference 添加用户偏好。
func (m *Manager) AddPreference(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prefs[key] = UserPreference{Key: key, Value: value}
}

// GetPreferences 获取所有用户偏好。
func (m *Manager) GetPreferences() []UserPreference {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var prefs []UserPreference
	for _, p := range m.prefs {
		prefs = append(prefs, p)
	}
	return prefs
}

// PreferencePrompt 将偏好注入 Prompt 字符串。
func PreferencePrompt(prefs []UserPreference) string {
	if len(prefs) == 0 {
		return ""
	}
	var parts []string
	for _, p := range prefs {
		parts = append(parts, fmt.Sprintf("- %s：%s", p.Key, p.Value))
	}
	return "\n\n用户偏好（回答时请注意）：\n" + stringsJoin(parts)
}

func stringsJoin(parts []string) string {
	var b []byte
	for i, p := range parts {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, p...)
	}
	return string(b)
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

// ConversationMemory 构造对话记忆文本（用于注入 system prompt）。
func ConversationMemory(turns []SessionTurn, maxTurns int) string {
	if len(turns) == 0 {
		return ""
	}
	display := turns
	if maxTurns > 0 && len(turns) > maxTurns {
		display = turns[len(turns)-maxTurns:]
	}
	var b []byte
	b = append(b, "\n\n对话历史："...)
	for _, t := range display {
		b = append(b, "\n用户: "...)
		b = append(b, t.Question...)
		b = append(b, "\n助手: "...)
		b = append(b, t.Answer...)
		b = append(b, '\n')
	}
	return string(b)
}
