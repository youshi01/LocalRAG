package service

import (
	"encoding/json"
	"localrag/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestThinkingSwitchDoesNotChangeOllamaModel(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[enabled], func(t *testing.T) {
			var payload ollamaChatRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("decode request: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"model": "configured-chat-model", "message": map[string]string{"role": "assistant", "content": "ok"}, "done": true})
			}))
			defer server.Close()
			svc := &LLMService{client: server.Client()}
			_, err := svc.Chat(model.ChatCompletionRequest{
				Model: "obsolete-thinking-model", Think: &enabled,
				Config:   model.ChatModelConfig{Provider: "ollama", BaseURL: server.URL, Model: "configured-chat-model"},
				Messages: []model.ChatMessage{{Role: "user", Content: "hello"}},
			})
			if err != nil {
				t.Fatalf("chat: %v", err)
			}
			if payload.Model != "configured-chat-model" {
				t.Fatalf("model changed: %s", payload.Model)
			}
			if payload.Think == nil || *payload.Think != enabled {
				t.Fatalf("thinking switch not forwarded: %+v", payload.Think)
			}
		})
	}
}

func TestUnsupportedCompatibleThinkingIsNotSilentlyIgnored(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "chat", true: "stream"}[stream], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"model": "configured-chat-model", "choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": "ok"}}}})
			}))
			defer server.Close()
			enabled := true
			req := model.ChatCompletionRequest{Think: &enabled, Config: model.ChatModelConfig{Provider: "openai-compatible", BaseURL: server.URL, Model: "configured-chat-model"}, Messages: []model.ChatMessage{{Role: "user", Content: "hello"}}}
			svc := &LLMService{client: server.Client(), streamClient: server.Client()}
			var err error
			if stream {
				err = svc.StreamChat(req, func(string) error { return nil })
			} else {
				_, err = svc.Chat(req)
			}
			if err == nil || !strings.Contains(err.Error(), "思考开关") {
				t.Errorf("expected an explicit unsupported-thinking error, got %v", err)
			}
			if calls.Load() != 0 {
				t.Errorf("unexpected upstream request silently ignoring thinking: %d", calls.Load())
			}
		})
	}
}
