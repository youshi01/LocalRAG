package handler

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"localrag/internal/model"
	"localrag/internal/service"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func inventoryTestHandler(t *testing.T) (*AppHandler, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "model must not be used for a directory", http.StatusBadRequest)
	}))
	t.Cleanup(upstream.Close)
	dir := t.TempDir()
	cfg := model.ServerConfig{StateFile: filepath.Join(dir, "state.json"), UploadDir: filepath.Join(dir, "uploads")}
	state := map[string]any{"config": model.AppConfig{Chat: model.ChatConfig{Provider: "openai-compatible", BaseURL: upstream.URL, Model: "unavailable-model", ContextMessageLimit: 12}}, "knowledgeBases": map[string]model.KnowledgeBase{
		"kb":    {ID: "kb", Name: "目录测试", Documents: []model.Document{{ID: "pdf", KnowledgeBaseID: "kb", Name: "assets.pdf", Status: "indexed"}, {ID: "xlsx", KnowledgeBaseID: "kb", Name: "rules.xlsx", Status: "failed"}, {ID: "md", KnowledgeBaseID: "kb", Name: "spec.md", Status: "ready"}}},
		"empty": {ID: "empty", Name: "空库", Documents: []model.Document{}},
		"other": {ID: "other", Name: "其他库", Documents: []model.Document{{ID: "secret", KnowledgeBaseID: "other", Name: "unrelated.txt"}}},
	}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(cfg.StateFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	history, err := service.NewSQLiteChatHistoryStore(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	app := service.NewAppService(nil, service.NewAppStateStore(cfg.StateFile), history, cfg)
	t.Cleanup(func() { _ = history.Close() })
	return NewAppHandler(cfg, app, service.NewLLMService()), &calls
}

func requestInventory(t *testing.T, h *AppHandler, stream bool, kb, doc, query string, think bool) *httptest.ResponseRecorder {
	t.Helper()
	engine := gin.New()
	if stream {
		engine.POST("/chat", h.ChatCompletionsStream)
	} else {
		engine.POST("/chat", h.ChatCompletions)
	}
	data, _ := json.Marshal(model.ChatCompletionRequest{ConversationID: "inventory-test", KnowledgeBaseID: kb, DocumentID: doc, Think: &think, Messages: []model.ChatMessage{{Role: "user", Content: query}}})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}

func TestDocumentInventoryReturnsAllFilesWithoutModel(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, think := range []bool{false, true} {
			t.Run(map[bool]string{false: "chat", true: "stream"}[stream]+map[bool]string{false: "-normal", true: "-think"}[think], func(t *testing.T) {
				h, calls := inventoryTestHandler(t)
				res := requestInventory(t, h, stream, "kb", "", "列出知识库里面的文件", think)
				if res.Code != 200 {
					t.Fatalf("expected deterministic 200, got %d: %s", res.Code, res.Body.String())
				}
				for _, name := range []string{"assets.pdf", "rules.xlsx", "spec.md"} {
					if !strings.Contains(res.Body.String(), name) {
						t.Errorf("missing file %s", name)
					}
				}
				if strings.Contains(res.Body.String(), "unrelated.txt") {
					t.Error("escaped knowledge base scope")
				}
				if calls.Load() != 0 {
					t.Errorf("directory request called model %d times", calls.Load())
				}
				if stream {
					if !strings.Contains(res.Body.String(), "event:done") {
						t.Error("missing done event")
					}
				} else {
					var out model.ChatCompletionResponse
					if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					support := out.Metadata["citationSupport"].(map[string]any)
					if support["basis"] != "document_inventory" || support["status"] != "supported" || support["supportedClaimCount"] != float64(3) {
						t.Fatalf("wrong metadata support: %#v", support)
					}
				}
			})
		}
	}
}

func TestDocumentInventoryCountsAllFiles(t *testing.T) {
	for _, q := range []string{"这个知识库有几个文件？", "当前知识库包含多少份文档？", "显示所有文件名", "list files in this knowledge base"} {
		t.Run(q, func(t *testing.T) {
			h, c := inventoryTestHandler(t)
			r := requestInventory(t, h, false, "kb", "", q, false)
			if r.Code != 200 || !strings.Contains(r.Body.String(), "3") {
				t.Fatalf("wrong directory count: %d %s", r.Code, r.Body.String())
			}
			if c.Load() != 0 {
				t.Error("used model")
			}
		})
	}
}

func TestDocumentInventoryHonorsDocumentScopeAndEmptyLibrary(t *testing.T) {
	t.Run("selected document", func(t *testing.T) {
		h, c := inventoryTestHandler(t)
		r := requestInventory(t, h, false, "kb", "md", "列出知识库里面的文件", false)
		if r.Code != 200 || !strings.Contains(r.Body.String(), "spec.md") || strings.Contains(r.Body.String(), "assets.pdf") {
			t.Fatalf("wrong document scope: %d %s", r.Code, r.Body.String())
		}
		if c.Load() != 0 {
			t.Error("used model")
		}
	})
	t.Run("empty library", func(t *testing.T) {
		h, c := inventoryTestHandler(t)
		r := requestInventory(t, h, false, "empty", "", "列出知识库里面的文件", false)
		if r.Code != 200 || !strings.Contains(r.Body.String(), "0") {
			t.Fatalf("wrong empty directory: %d %s", r.Code, r.Body.String())
		}
		if c.Load() != 0 {
			t.Error("used model")
		}
	})
	t.Run("invalid scope", func(t *testing.T) {
		h, c := inventoryTestHandler(t)
		r := requestInventory(t, h, false, "kb", "secret", "列出知识库里面的文件", false)
		if r.Code == 200 {
			t.Error("accepted document from another library")
		}
		if c.Load() != 0 {
			t.Error("used model for invalid scope")
		}
	})
}

func TestDocumentInventoryDoesNotInterceptContentQuestions(t *testing.T) {
	for _, q := range []string{"文件中有哪些接口？", "列出文档里面的关键结论", "请总结知识库中的文件内容", "知识库目录查询功能怎么实现？", "这个文件有多少页？", "文档包含多少条记录？", "统计用户profile数量"} {
		t.Run(q, func(t *testing.T) {
			h, c := inventoryTestHandler(t)
			requestInventory(t, h, false, "kb", "", q, false)
			if c.Load() == 0 {
				t.Fatal("content question incorrectly routed as file inventory")
			}
		})
	}
}

func TestDocumentInventoryRegenerateAlsoUsesDirectory(t *testing.T) {
	h, calls := inventoryTestHandler(t)
	first := requestInventory(t, h, false, "kb", "", "列出知识库里面的文件", false)
	if first.Code != 200 {
		t.Fatalf("initial directory response: %d", first.Code)
	}
	conversation, err := h.appService.GetConversation("inventory-test")
	if err != nil {
		t.Fatal(err)
	}
	messageID := conversation.Messages[len(conversation.Messages)-1].ID
	engine := gin.New()
	engine.POST("/conversations/:id/messages/:msgId/regenerate", h.RegenerateMessage)
	request := httptest.NewRequest(http.MethodPost, "/conversations/inventory-test/messages/"+messageID+"/regenerate", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("regenerate: %d %s", response.Code, response.Body.String())
	}
	for _, name := range []string{"assets.pdf", "rules.xlsx", "spec.md"} {
		if !strings.Contains(response.Body.String(), name) {
			t.Errorf("missing %s", name)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("regeneration used model: %d", calls.Load())
	}
}
