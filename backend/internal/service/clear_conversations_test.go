package service

import (
	"localrag/internal/model"
	"path/filepath"
	"testing"
)

type testBulkConversationStore interface{ DeleteAllConversations() (int, error) }

func seedBulkHistory(t *testing.T, store *SQLiteChatHistoryStore) {
	t.Helper()
	for _, id := range []string{"one", "two", "three"} {
		if err := store.SaveConversation(model.Conversation{ID: id, Title: id, KnowledgeBaseID: "kb-" + id, Messages: []model.StoredChatMessage{{ID: id + "-user", Role: "user", Content: "question"}, {ID: id + "-assistant", Role: "assistant", Content: "answer"}}}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestClearAllConversationsPersistsAndDeletesMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := NewSQLiteChatHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	seedBulkHistory(t, store)
	clear, ok := any(store).(testBulkConversationStore)
	if !ok {
		t.Fatal("missing atomic bulk conversation deletion")
	}
	count, err := clear.DeleteAllConversations()
	if err != nil || count != 3 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	var rows int
	if err = store.db.QueryRow("SELECT count(*) FROM messages").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("messages remain: %d %v", rows, err)
	}
	reopened, err := NewSQLiteChatHistoryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.ListConversations()
	if err != nil || len(items) != 0 {
		t.Fatalf("deleted sessions reappeared: %+v %v", items, err)
	}
	count, err = clear.DeleteAllConversations()
	if err != nil || count != 0 {
		t.Fatalf("empty clear is not idempotent: %d %v", count, err)
	}
}
func TestClearAllConversationsRollsBackOnFailure(t *testing.T) {
	store, err := NewSQLiteChatHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	seedBulkHistory(t, store)
	clear, ok := any(store).(testBulkConversationStore)
	if !ok {
		t.Fatal("missing atomic bulk conversation deletion")
	}
	_, err = store.db.Exec("CREATE TRIGGER prevent_delete BEFORE DELETE ON conversations BEGIN SELECT RAISE(ABORT, 'blocked'); END;")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = clear.DeleteAllConversations(); err == nil {
		t.Fatal("expected trigger failure")
	}
	var conversations, messages int
	_ = store.db.QueryRow("SELECT count(*) FROM conversations").Scan(&conversations)
	_ = store.db.QueryRow("SELECT count(*) FROM messages").Scan(&messages)
	if conversations != 3 || messages != 6 {
		t.Fatalf("partial deletion: conversations=%d messages=%d", conversations, messages)
	}
}
