package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"localrag/internal/model"
)

func TestGetPublicConfigRedactsSecrets(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{
		EnableMCP:            true,
		EnableMCPLegacyToken: true,
	})

	cfg := service.GetConfig()
	cfg.Chat.APIKey = "chat-secret"
	cfg.Embedding.APIKey = "embedding-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with secrets: %v", err)
	}

	public := service.GetPublicConfig()
	if public.Chat.APIKey != "" || public.Embedding.APIKey != "" || public.MCP.Token != "" {
		t.Fatalf("expected public config secrets to be redacted, got %+v", public)
	}
	if !public.Chat.APIKeyConfigured || !public.Embedding.APIKeyConfigured || !public.MCP.TokenConfigured {
		t.Fatalf("expected configured secret flags, got %+v", public)
	}
	if !public.MCP.LegacyTokenEnabled {
		t.Fatalf("expected public config to expose legacy token enabled status")
	}
}

func TestAuthDeploymentWarningsOnlyIncludeSetupWarningBeforeRootExists(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{
		EnableAuth: true,
	})

	if warnings := service.AuthDeploymentWarnings(); len(warnings) != 1 {
		t.Fatalf("expected setup warning before root exists, got %v", warnings)
	}

	service.state.Mu.Lock()
	service.state.Auth.Users["usr_root"] = model.AuthUser{
		ID:       "usr_root",
		Username: "root",
		Role:     "root",
	}
	service.state.Mu.Unlock()

	if warnings := service.AuthDeploymentWarnings(); len(warnings) != 0 {
		t.Fatalf("expected no setup warning after root exists, got %v", warnings)
	}
}

func TestUpdateConfigPreservesConfiguredSecretsWhenPublicConfigIsSaved(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{
		EnableMCP:            true,
		EnableMCPLegacyToken: true,
	})

	cfg := service.GetConfig()
	cfg.Chat.APIKey = "chat-secret"
	cfg.Embedding.APIKey = "embedding-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with secrets: %v", err)
	}

	public := service.GetPublicConfig()
	public.Chat.Model = "updated-chat-model"
	public.Embedding.Model = "updated-embedding-model"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(public)); err != nil {
		t.Fatalf("update config from public config: %v", err)
	}

	internal := service.GetConfig()
	if internal.Chat.APIKey != "chat-secret" {
		t.Fatalf("expected chat secret to be preserved, got %q", internal.Chat.APIKey)
	}
	if internal.Embedding.APIKey != "embedding-secret" {
		t.Fatalf("expected embedding secret to be preserved, got %q", internal.Embedding.APIKey)
	}
	if internal.MCP.Token == "" {
		t.Fatalf("expected mcp token to be preserved")
	}
	if internal.Chat.Model != "updated-chat-model" || internal.Embedding.Model != "updated-embedding-model" {
		t.Fatalf("expected non-secret config fields to update, got %+v", internal)
	}
}

func TestUpdateConfigDoesNotCarryChatSecretAcrossEndpointChange(t *testing.T) {
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"model":"chat-model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
	}))
	t.Cleanup(upstream.Close)

	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{})
	cfg := service.GetConfig()
	cfg.Chat.Provider = "openai-compatible"
	cfg.Chat.BaseURL = "http://127.0.0.1:19000/v1"
	cfg.Chat.APIKey = "chat-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with chat secret: %v", err)
	}

	public := service.GetPublicConfig()
	public.Chat.BaseURL = upstream.URL
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(public)); err != nil {
		t.Fatalf("update config after chat endpoint change: %v", err)
	}

	if got := service.GetConfig().Chat.APIKey; got != "" {
		t.Fatalf("expected chat secret to be cleared for a new endpoint, got %q", got)
	}

	response, err := NewLLMService().Chat(model.ChatCompletionRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "hello"}},
		Config:   service.CurrentChatConfig(),
	})
	if err != nil || len(response.Choices) != 1 {
		t.Fatalf("expected chat request against the new endpoint to succeed, response=%#v err=%v", response, err)
	}
	if authorization != "" {
		t.Fatalf("expected old chat secret not to reach the new endpoint, got authorization %q", authorization)
	}
}

func TestUpdateConfigDoesNotCarryEmbeddingSecretAcrossEndpointChange(t *testing.T) {
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`)
	}))
	t.Cleanup(upstream.Close)

	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{})
	cfg := service.GetConfig()
	cfg.Embedding.Provider = "openai-compatible"
	cfg.Embedding.BaseURL = "http://127.0.0.1:19000/v1"
	cfg.Embedding.APIKey = "embedding-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with embedding secret: %v", err)
	}

	public := service.GetPublicConfig()
	public.Embedding.BaseURL = upstream.URL
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(public)); err != nil {
		t.Fatalf("update config after embedding endpoint change: %v", err)
	}

	if got := service.GetConfig().Embedding.APIKey; got != "" {
		t.Fatalf("expected embedding secret to be cleared for a new endpoint, got %q", got)
	}

	config := service.CurrentEmbeddingConfig()
	probe, err := (&ModelService{client: upstream.Client()}).Probe(t.Context(), model.ModelProbeRequest{
		Type:     model.ModelKindEmbedding,
		Provider: config.Provider,
		BaseURL:  config.BaseURL,
		Model:    config.Model,
		APIKey:   config.APIKey,
	}, 3)
	if err != nil || !probe.Success {
		t.Fatalf("expected embedding probe against the new endpoint to succeed, probe=%#v err=%v", probe, err)
	}
	if authorization != "" {
		t.Fatalf("expected old embedding secret not to reach the new endpoint, got authorization %q", authorization)
	}
}

func TestUpdateConfigClearsConfiguredSecretsWhenExplicitlyRequested(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{
		EnableMCP:            true,
		EnableMCPLegacyToken: true,
	})

	cfg := service.GetConfig()
	cfg.Chat.APIKey = "chat-secret"
	cfg.Embedding.APIKey = "embedding-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with secrets: %v", err)
	}

	public := service.GetPublicConfig()
	public.Chat.ClearAPIKey = true
	public.Embedding.ClearAPIKey = true
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(public)); err != nil {
		t.Fatalf("clear configured secrets: %v", err)
	}

	internal := service.GetConfig()
	if internal.Chat.APIKey != "" || internal.Chat.APIKeyConfigured {
		t.Fatalf("expected chat secret to be cleared, got %+v", internal.Chat)
	}
	if internal.Embedding.APIKey != "" || internal.Embedding.APIKeyConfigured {
		t.Fatalf("expected embedding secret to be cleared, got %+v", internal.Embedding)
	}
}

func TestRequestConfigUsesStoredSecretOnlyForConfiguredEndpoint(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{})
	cfg := service.GetConfig()
	cfg.Chat.APIKey = "chat-secret"
	cfg.Embedding.APIKey = "embedding-secret"
	if _, err := service.UpdateConfig(model.ConfigUpdateRequest(cfg)); err != nil {
		t.Fatalf("update config with secrets: %v", err)
	}

	request := model.ChatCompletionRequest{
		Config: model.ChatModelConfig{
			Provider: cfg.Chat.Provider,
			BaseURL:  cfg.Chat.BaseURL,
			Model:    "another-chat-model",
		},
		Embedding: model.EmbeddingModelConfig{
			Provider: cfg.Embedding.Provider,
			BaseURL:  cfg.Embedding.BaseURL,
			Model:    "another-embedding-model",
		},
	}
	if got := service.resolveChatConfig(request).APIKey; got != "chat-secret" {
		t.Fatalf("expected stored chat secret for configured endpoint, got %q", got)
	}
	if got := service.resolveEmbeddingConfig(request).APIKey; got != "embedding-secret" {
		t.Fatalf("expected stored embedding secret for configured endpoint, got %q", got)
	}

	request.Config.BaseURL = "https://untrusted.example/v1"
	request.Embedding.BaseURL = "https://untrusted.example/v1"
	if got := service.resolveChatConfig(request).APIKey; got != "" {
		t.Fatalf("expected no chat secret for an alternate endpoint, got %q", got)
	}
	if got := service.resolveEmbeddingConfig(request).APIKey; got != "" {
		t.Fatalf("expected no embedding secret for an alternate endpoint, got %q", got)
	}
}

func TestUpdateConfigCanonicalizesModelProviderAndBaseURL(t *testing.T) {
	service := NewAppService(nil, NewAppStateStore(""), nil, model.ServerConfig{})
	_, err := service.UpdateConfig(model.ConfigUpdateRequest{
		Chat: model.ChatConfig{
			Provider: " OPENAI ",
			BaseURL:  "http://127.0.0.1:9000/",
			Model:    "chat-model",
		},
		Embedding: model.EmbeddingConfig{
			Provider: "openai-compatible",
			BaseURL:  "http://127.0.0.1:9000/v1/",
			Model:    "embedding-model",
		},
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}

	config := service.GetConfig()
	if config.Chat.Provider != "openai-compatible" || config.Chat.BaseURL != "http://127.0.0.1:9000/v1" {
		t.Fatalf("expected canonical chat endpoint, got %#v", config.Chat)
	}
	if config.Embedding.Provider != "openai-compatible" || config.Embedding.BaseURL != "http://127.0.0.1:9000/v1" {
		t.Fatalf("expected canonical embedding endpoint, got %#v", config.Embedding)
	}
}
