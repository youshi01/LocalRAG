package handler

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"localrag/internal/model"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testClearAllHandler interface{ DeleteAllConversations(*gin.Context) }

func clearConversationsRequest(t *testing.T, h *AppHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	clear, ok := any(h).(testClearAllHandler)
	if !ok {
		t.Fatal("missing clear-all endpoint handler")
	}
	engine := gin.New()
	engine.DELETE("/api/conversations", clear.DeleteAllConversations)
	req := httptest.NewRequest(http.MethodDelete, "/api/conversations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}
func seedClearHandler(t *testing.T, h *AppHandler) {
	t.Helper()
	for _, id := range []string{"clear-one", "clear-two"} {
		_, err := h.appService.SaveConversation(model.SaveConversationRequest{ID: id, KnowledgeBaseID: "kb", Messages: []model.StoredChatMessage{{ID: id + "-user", Role: "user", Content: "keep until confirmed"}}})
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestClearAllConversationsRequiresConfirmationAndPreservesKnowledge(t *testing.T) {
	h, _ := inventoryTestHandler(t)
	seedClearHandler(t, h)
	for _, body := range []string{"", "{}", "{\"confirm\":false}"} {
		r := clearConversationsRequest(t, h, body)
		if r.Code != 400 {
			t.Fatalf("unconfirmed delete accepted: %d", r.Code)
		}
		items, _ := h.appService.ListConversations()
		if len(items) != 2 {
			t.Fatal("unconfirmed request deleted sessions")
		}
	}
	r := clearConversationsRequest(t, h, "{\"confirm\":true}")
	if r.Code != 200 {
		t.Fatalf("clear: %d %s", r.Code, r.Body.String())
	}
	var result map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &result)
	if result["deletedCount"] != float64(2) {
		t.Fatalf("wrong count: %#v", result)
	}
	items, _ := h.appService.ListConversations()
	if len(items) != 0 {
		t.Fatal("sessions remain")
	}
	inv, ok, err := h.appService.BuildDocumentInventoryAnswer(model.ChatCompletionRequest{KnowledgeBaseID: "kb", Messages: []model.ChatMessage{{Role: "user", Content: "列出文件"}}})
	if err != nil || !ok || inv.Count != 3 {
		t.Fatalf("knowledge data changed: %+v %v", inv, err)
	}
}
func TestClearAllConversationsRejectsInFlightGeneration(t *testing.T) {
	h, _ := inventoryTestHandler(t)
	seedClearHandler(t, h)
	guard, ok := any(h.appService).(interface{ BeginConversationRequest() func() })
	if !ok {
		t.Fatal("missing in-flight conversation protection")
	}
	release := guard.BeginConversationRequest()
	r := clearConversationsRequest(t, h, "{\"confirm\":true}")
	release()
	if r.Code != 409 {
		t.Fatalf("cleared during generation: %d %s", r.Code, r.Body.String())
	}
	items, _ := h.appService.ListConversations()
	if len(items) != 2 {
		t.Fatal("active clear deleted sessions")
	}
	r = clearConversationsRequest(t, h, "{\"confirm\":true}")
	if r.Code != 200 {
		t.Fatalf("clear after completion: %d %s", r.Code, r.Body.String())
	}
}
