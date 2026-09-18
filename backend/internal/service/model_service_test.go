package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"localrag/internal/model"
)

func TestNormalizeModelEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		baseURL  string
		want     string
		wantErr  bool
	}{
		{name: "ollama removes v1", provider: "ollama", baseURL: "http://127.0.0.1:11434/v1/", want: "http://127.0.0.1:11434"},
		{name: "openai adds v1", provider: "openai-compatible", baseURL: "http://127.0.0.1:9000", want: "http://127.0.0.1:9000/v1"},
		{name: "openai keeps v1", provider: "openai", baseURL: "http://127.0.0.1:9000/v1/", want: "http://127.0.0.1:9000/v1"},
		{name: "rejects endpoint URL", provider: "openai-compatible", baseURL: "http://127.0.0.1:9000/v1/models", wantErr: true},
		{name: "rejects missing scheme", provider: "ollama", baseURL: "127.0.0.1:11434", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, normalized, err := normalizeModelEndpoint(tc.provider, tc.baseURL)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected endpoint validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize endpoint: %v", err)
			}
			if normalized != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, normalized)
			}
		})
	}
}

func TestNormalizeModelProviderAcceptsLegacyOpenAIName(t *testing.T) {
	provider, err := normalizeModelProvider(" OPENAI ")
	if err != nil {
		t.Fatalf("normalize provider: %v", err)
	}
	if provider != "openai-compatible" {
		t.Fatalf("expected openai-compatible, got %q", provider)
	}
}

func TestNormalizeModelEndpointRejectsUserInfoWithoutLeakingSecret(t *testing.T) {
	const secret = "endpoint-secret"

	_, _, err := normalizeModelEndpoint("ollama", "http://user:"+secret+"@127.0.0.1:11434/v1")
	if err == nil {
		t.Fatal("expected userinfo endpoint validation error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("endpoint validation error leaked secret: %q", err)
	}
}

func TestNormalizeModelEndpointRejectsQueryAndFragment(t *testing.T) {
	for _, suffix := range []string{"?api_key=should-not-be-sent", "#fragment"} {
		t.Run(suffix, func(t *testing.T) {
			_, _, err := normalizeModelEndpoint("ollama", "http://127.0.0.1:11434"+suffix)
			if err == nil {
				t.Fatal("expected query or fragment endpoint validation error")
			}
			if strings.Contains(err.Error(), suffix) || strings.Contains(err.Error(), "should-not-be-sent") {
				t.Fatalf("endpoint validation error leaked URL data: %q", err)
			}
		})
	}
}

func TestNormalizeModelKindRejectsUnsupportedValues(t *testing.T) {
	for _, kind := range []model.ModelKind{"audio", "invalid"} {
		if _, err := normalizeModelKind(kind); err == nil {
			t.Fatalf("expected model kind %q validation error", kind)
		}
	}
}

func TestModelServiceRejectsUnsupportedModelKinds(t *testing.T) {
	service := NewModelService()
	for _, kind := range []model.ModelKind{"audio", "invalid"} {
		if _, err := service.ListModels(context.Background(), model.ModelListRequest{Type: kind}); err == nil {
			t.Fatalf("expected ListModels to reject model kind %q", kind)
		}
		if _, err := service.Probe(context.Background(), model.ModelProbeRequest{Type: kind}, 0); err == nil {
			t.Fatalf("expected Probe to reject model kind %q", kind)
		}
	}
}

func TestListModelsReadsOllamaTagsAndMeasuresLatency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"nomic-embed-text"},{"model":"qwen3.5:9b"}]}`)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "ollama", BaseURL: server.URL,
	})
	if err != nil || !result.Success {
		t.Fatalf("list models: result=%#v err=%v", result, err)
	}
	if result.LatencyMs < 0 || len(result.Models) != 2 {
		t.Fatalf("expected two models and non-negative latency, got %#v", result)
	}
	if result.Models[0].Type != model.ModelKindChat {
		t.Fatalf("expected requested model type, got %#v", result.Models[0])
	}
}

func TestListModelsReadsOpenAIModelsWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer discovery-secret" {
			t.Fatalf("unexpected request: path=%s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"chat-model","owned_by":"test"}]}`)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindEmbedding, Provider: "openai-compatible", BaseURL: server.URL,
		APIKey: "discovery-secret",
	})
	if err != nil || !result.Success || len(result.Models) != 1 {
		t.Fatalf("list models: result=%#v err=%v", result, err)
	}
	if result.Models[0].OwnedBy != "test" || result.Models[0].Type != model.ModelKindEmbedding {
		t.Fatalf("unexpected model option: %#v", result.Models[0])
	}
}

func TestListModelsNeverReturnsUpstreamBodyOrAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"secret-discovery-key"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL,
		APIKey: "secret-discovery-key",
	})
	if err != nil {
		t.Fatalf("expected safe response instead of transport error: %v", err)
	}
	if result.Success || result.ErrorCode != "authentication_failed" {
		t.Fatalf("expected authentication failure, got %#v", result)
	}
	if strings.Contains(result.ErrorMessage, "secret-discovery-key") {
		t.Fatalf("error leaked secret: %q", result.ErrorMessage)
	}
}

func TestListModelsRejectsTrailingJSONValues(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		body     string
	}{
		{name: "ollama", provider: "ollama", body: `{"models":[]}garbage`},
		{name: "openai", provider: "openai-compatible", body: `{"data":[]} {"data":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)

			result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
				Type: model.ModelKindChat, Provider: tc.provider, BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("list models: %v", err)
			}
			if result.Success || result.ErrorCode != "invalid_response" || result.ErrorMessage != "模型列表响应格式无效" {
				t.Fatalf("expected invalid response, got %#v", result)
			}
		})
	}
}

func TestListModelsRejectsResponseBeyondSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`+strings.Repeat(" ", 4<<20))
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if result.Success || result.ErrorCode != "invalid_response" || result.ErrorMessage != "模型列表响应格式无效" {
		t.Fatalf("expected oversized response to be invalid, got %#v", result)
	}
}

func TestListModelsDeduplicatesAndSortsIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"z-model"}]}`)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL,
	})
	if err != nil || !result.Success || len(result.Models) != 2 {
		t.Fatalf("list models: result=%#v err=%v", result, err)
	}
	if result.Models[0].ID != "a-model" || result.Models[1].ID != "z-model" {
		t.Fatalf("expected sorted unique IDs, got %#v", result.Models)
	}
}

func TestListModelsReturnsNonNilEmptyModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"models":[]}`)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "ollama", BaseURL: server.URL,
	})
	if err != nil || !result.Success {
		t.Fatalf("list models: result=%#v err=%v", result, err)
	}
	if result.Models == nil || len(result.Models) != 0 {
		t.Fatalf("expected a non-nil empty model list, got %#v", result.Models)
	}
}

func TestProbeOllamaChatSendsExpectedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Model    string              `json:"model"`
			Messages []model.ChatMessage `json:"messages"`
			Stream   bool                `json:"stream"`
			Think    *bool               `json:"think"`
			Options  struct {
				Temperature float64 `json:"temperature"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Model != "qwen3.5:9b" || payload.Stream || payload.Think == nil || *payload.Think || payload.Options.Temperature != 0.25 {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "Please reply with OK only." {
			t.Fatalf("unexpected probe message: %#v", payload.Messages)
		}
		_, _ = io.WriteString(w, `{"model":"qwen3.5:9b","message":{"role":"assistant","content":"OK"}}`)
	}))
	t.Cleanup(server.Close)
	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindChat, Provider: "ollama", BaseURL: server.URL, Model: "qwen3.5:9b", Temperature: 0.25}, 768)
	if err != nil || !result.Success || result.Model != "qwen3.5:9b" {
		t.Fatalf("probe chat: result=%#v err=%v", result, err)
	}
	if result.ExpectedVectorSize != 0 || result.DimensionMatch != nil {
		t.Fatalf("expected chat probe to omit embedding dimensions, got %#v", result)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %d", result.LatencyMs)
	}
}

func TestProbeRejectsOutOfRangeChatTemperature(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"model":"test-model","message":{"role":"assistant","content":"OK"}}`)
	}))
	t.Cleanup(server.Close)

	for _, temperature := range []float64{-0.01, 2.01} {
		result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{
			Type: model.ModelKindChat, Provider: "ollama", BaseURL: server.URL, Model: "test-model", Temperature: temperature,
		}, 0)
		if err == nil {
			t.Fatalf("expected temperature %.2f to be rejected, result=%#v", temperature, result)
		}
	}
	if calls != 0 {
		t.Fatalf("expected invalid temperature probes not to call upstream, got %d calls", calls)
	}
}

func TestProbeOpenAICompatibleChatSendsBearerAndExpectedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer probe-secret" {
			t.Fatalf("unexpected request: method=%s path=%s authorization=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		var payload struct {
			Model       string              `json:"model"`
			Messages    []model.ChatMessage `json:"messages"`
			Stream      bool                `json:"stream"`
			Temperature float64             `json:"temperature"`
			MaxTokens   int                 `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Model != "gpt-test" || payload.Stream || payload.Temperature != 0.5 || payload.MaxTokens != 16 {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "Please reply with OK only." {
			t.Fatalf("unexpected probe message: %#v", payload.Messages)
		}
		_, _ = io.WriteString(w, `{"model":"returned-model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
	}))
	t.Cleanup(server.Close)
	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL, Model: "gpt-test", APIKey: "probe-secret", Temperature: 0.5}, 0)
	if err != nil || !result.Success || result.Model != "returned-model" {
		t.Fatalf("probe chat: result=%#v err=%v", result, err)
	}
}

func TestProbeOllamaEmbeddingReturnsDimensionAndLatency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/embed" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Model != "nomic-embed-text" || len(payload.Input) != 1 || payload.Input[0] != "LocalRAG model health probe" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		_, _ = io.WriteString(w, `{"embeddings":[[0.1,0.2,0.3]]}`)
	}))
	t.Cleanup(server.Close)
	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindEmbedding, Provider: "ollama", BaseURL: server.URL, Model: "nomic-embed-text"}, 3)
	if err != nil || !result.Success || result.VectorSize != 3 || result.ExpectedVectorSize != 3 {
		t.Fatalf("probe embedding: result=%#v err=%v", result, err)
	}
	if result.DimensionMatch == nil || *result.DimensionMatch != true {
		t.Fatalf("expected dimension match marker, got %#v", result.DimensionMatch)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %d", result.LatencyMs)
	}
}

func TestProbeOpenAICompatibleEmbeddingReportsDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer embed-secret" {
			t.Fatalf("unexpected request: method=%s path=%s authorization=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		var payload struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Model != "text-embedding" || len(payload.Input) != 1 || payload.Input[0] != "LocalRAG model health probe" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		_, _ = io.WriteString(w, `{"data":[{"index":0,"embedding":[0.1,0.2]}]}`)
	}))
	t.Cleanup(server.Close)
	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindEmbedding, Provider: "openai-compatible", BaseURL: server.URL, Model: "text-embedding", APIKey: "embed-secret"}, 3)
	if err != nil || result.Success || result.ErrorCode != "dimension_mismatch" || result.VectorSize != 2 || result.ExpectedVectorSize != 3 {
		t.Fatalf("expected dimension mismatch, got result=%#v err=%v", result, err)
	}
	if result.DimensionMatch == nil || *result.DimensionMatch != false {
		t.Fatalf("expected dimension mismatch marker, got %#v", result.DimensionMatch)
	}
}

func TestProbeExplainsEmbeddingCapabilityError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"This server does not support embeddings. Start it with --embeddings"}}`)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{
		Type: model.ModelKindEmbedding, Provider: "openai-compatible", BaseURL: server.URL, Model: "remote-embedding",
	}, 3)
	if err != nil {
		t.Fatalf("probe returned unexpected error: %v", err)
	}
	if result.Success || result.ErrorCode != "embedding_not_supported" {
		t.Fatalf("expected embedding capability failure, got %#v", result)
	}
	if !strings.Contains(result.ErrorMessage, "Embedding 服务未启用向量接口") || !strings.Contains(result.ErrorMessage, "--embeddings") {
		t.Fatalf("expected actionable Chinese embedding capability guidance, got %q", result.ErrorMessage)
	}
}

func TestProbeExplainsPlainTextEmbeddingCapabilityError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "This endpoint does not support embeddings. Start it with --embeddings")
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{
		Type: model.ModelKindEmbedding, Provider: "openai-compatible", BaseURL: server.URL, Model: "remote-embedding",
	}, 3)
	if err != nil || result.Success || result.ErrorCode != "embedding_not_supported" {
		t.Fatalf("expected embedding capability failure, result=%#v err=%v", result, err)
	}
	if !strings.Contains(result.ErrorMessage, "Embedding 服务未启用向量接口") {
		t.Fatalf("expected actionable capability guidance, got %q", result.ErrorMessage)
	}
}

func TestProbeMapsHTTP400ModelNotFoundWithOneRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		time.Sleep(10 * time.Millisecond)
		http.Error(w, `{"error":{"message":"model missing-model not found"}}`, http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL, Model: "missing-model"}, 0)
	if err != nil || result.Success || result.ErrorCode != "model_not_found" || calls != 1 {
		t.Fatalf("expected one model-not-found attempt, got result=%#v err=%v calls=%d", result, err, calls)
	}
	if result.LatencyMs <= 0 {
		t.Fatalf("expected recorded upstream-failure latency, got %d", result.LatencyMs)
	}
}

func TestProbeClassifiesParentDeadlineAsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(server.Close)
	parent, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	result, err := (&ModelService{client: server.Client()}).Probe(parent, model.ModelProbeRequest{Type: model.ModelKindChat, Provider: "ollama", BaseURL: server.URL, Model: "slow-model"}, 0)
	if err != nil || result.Success || result.ErrorCode != "timeout" {
		t.Fatalf("expected timeout, got result=%#v err=%v", result, err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("probe waited for default timeout")
	}
}

func TestProbeClassifiesEmptyAndInvalidResponses(t *testing.T) {
	cases := []struct{ name, body, wantCode string }{
		{name: "empty response", body: `{"choices":[{"message":{"content":" "}}]}`, wantCode: "empty_response"},
		{name: "invalid json", body: `{`, wantCode: "invalid_response"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) }))
			t.Cleanup(server.Close)
			result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL, Model: "gpt-test"}, 0)
			if err != nil || result.Success || result.ErrorCode != tc.wantCode {
				t.Fatalf("expected %s, got result=%#v err=%v", tc.wantCode, result, err)
			}
		})
	}
}

func TestProbeRejectsAnyEmptyEmbeddingVector(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		path     string
		body     string
	}{
		{name: "ollama", provider: "ollama", path: "/api/embed", body: `{"embeddings":[[0.1],[]]}`},
		{name: "openai", provider: "openai-compatible", path: "/v1/embeddings", body: `{"data":[{"index":0,"embedding":[0.1]},{"index":1,"embedding":[]}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{
				Type: model.ModelKindEmbedding, Provider: tc.provider, BaseURL: server.URL, Model: "embedding-test",
			}, 0)
			if err != nil || result.Success || result.ErrorCode != "empty_response" {
				t.Fatalf("expected empty embedding response, got result=%#v err=%v", result, err)
			}
		})
	}
}

func TestProbeDoesNotFollowRedirect(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls++
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"OK"}}]}`)
	}))
	t.Cleanup(target.Close)

	sourceCalls := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls++
		http.Redirect(w, r, target.URL+"/redirect-target", http.StatusFound)
	}))
	t.Cleanup(source.Close)

	result, err := (&ModelService{client: source.Client()}).Probe(t.Context(), model.ModelProbeRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: source.URL, Model: "redirect-model",
	}, 0)
	if err != nil || result.Success || result.ErrorCode != "upstream_error" || result.ErrorMessage != "模型服务请求失败" {
		t.Fatalf("expected safe redirect failure, got result=%#v err=%v", result, err)
	}
	if sourceCalls != 1 || targetCalls != 0 {
		t.Fatalf("expected exactly one source request and no target request, got source=%d target=%d", sourceCalls, targetCalls)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %d", result.LatencyMs)
	}
}

func TestProbeAuthenticationFailureIsSafe(t *testing.T) {
	const apiKey = "probe-auth-secret"
	const upstreamBody = "upstream-auth-body-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"`+upstreamBody+`"}}`, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	result, err := (&ModelService{client: server.Client()}).Probe(t.Context(), model.ModelProbeRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: server.URL, Model: "protected-model", APIKey: apiKey,
	}, 0)
	if err != nil || result.Success || result.ErrorCode != "authentication_failed" || result.ErrorMessage != "模型服务鉴权失败" {
		t.Fatalf("expected safe authentication failure, got result=%#v err=%v", result, err)
	}
	if strings.Contains(result.ErrorMessage, apiKey) || strings.Contains(result.ErrorMessage, upstreamBody) {
		t.Fatalf("probe error leaked sensitive data: %q", result.ErrorMessage)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %d", result.LatencyMs)
	}
}

func TestListModelsDoesNotFollowRedirect(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls++
		_, _ = io.WriteString(w, `{"data":[{"id":"redirected-model"}]}`)
	}))
	t.Cleanup(target.Close)

	sourceCalls := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls++
		http.Redirect(w, r, target.URL+"/redirect-target", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(source.Close)

	result, err := (&ModelService{client: source.Client()}).ListModels(t.Context(), model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "openai-compatible", BaseURL: source.URL,
	})
	if err != nil || result.Success || result.ErrorCode != "upstream_error" {
		t.Fatalf("expected redirect failure, got result=%#v err=%v", result, err)
	}
	if sourceCalls != 1 || targetCalls != 0 {
		t.Fatalf("expected exactly one source request and no target request, got source=%d target=%d", sourceCalls, targetCalls)
	}
}
