package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
