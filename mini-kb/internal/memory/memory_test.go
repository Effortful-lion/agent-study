package memory

import (
	"fmt"
	"testing"
)

func TestSession(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	session := mgr.LoadSession("test-session")
	if session.ID != "test-session" {
		t.Errorf("LoadSession got ID %q, want %q", session.ID, "test-session")
	}
	if session.TurnCount() != 0 {
		t.Error("new session should have 0 turns")
	}

	mgr.AddTurn(session, SessionTurn{
		Question: "Q1",
		Answer:   "A1",
		Sources:  []string{"doc1.md"},
	})
	if session.TurnCount() != 1 {
		t.Error("session should have 1 turn")
	}

	history := session.History(1)
	if len(history) != 1 {
		t.Errorf("History(1) got %d turns, want 1", len(history))
	}

	// Save and reload
	err := mgr.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession error: %v", err)
	}

	reloaded := mgr.LoadSession("test-session")
	if reloaded.TurnCount() != 1 {
		t.Errorf("reloaded session has %d turns, want 1", reloaded.TurnCount())
	}
	if reloaded.Turns[0].Question != "Q1" {
		t.Error("reloaded turn question mismatch")
	}
}

func TestPreferences(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	mgr.AddPreference("language", "Go")
	mgr.AddPreference("style", "code examples")

	prefs := mgr.GetPreferences()
	if len(prefs) != 2 {
		t.Errorf("GetPreferences got %d, want 2", len(prefs))
	}

	prompt := PreferencePrompt(prefs)
	if prompt == "" {
		t.Error("PreferencePrompt should not be empty")
	}
}

func TestConversationMemory(t *testing.T) {
	turns := []SessionTurn{
		{Question: "Q1", Answer: "A1"},
		{Question: "Q2", Answer: "A2"},
		{Question: "Q3", Answer: "A3"},
	}

	mem := ConversationMemory(turns, 2)
	if mem == "" {
		t.Error("ConversationMemory should not be empty")
	}

	// Should only include last 2 turns
	memFull := ConversationMemory(turns, 0)
	if len(memFull) <= len(mem) {
		t.Error("full memory should be longer than limited memory")
	}
}

func TestSessionHistoryLimit(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	session := mgr.LoadSession("hist-test")

	for i := 0; i < 10; i++ {
		mgr.AddTurn(session, SessionTurn{
			Question: fmt.Sprintf("Q%d", i),
			Answer:   fmt.Sprintf("A%d", i),
		})
	}

	history := session.History(3)
	if len(history) != 3 {
		t.Errorf("History(3) got %d turns, want 3", len(history))
	}
	if history[0].Question != "Q7" {
		t.Error("History should return last 3 turns")
	}
}

func TestSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	mgr1 := NewManager(dir)

	session := mgr1.LoadSession("persist-test")
	mgr1.AddTurn(session, SessionTurn{Question: "persist?", Answer: "yes"})
	mgr1.SaveSession(session)

	// Load from new manager instance
	mgr2 := NewManager(dir)
	reloaded := mgr2.LoadSession("persist-test")
	if reloaded.TurnCount() != 1 {
		t.Errorf("persisted session has %d turns, want 1", reloaded.TurnCount())
	}
}
