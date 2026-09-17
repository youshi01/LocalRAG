package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"localrag/internal/model"
	"localrag/internal/service"
)

func TestExpectedEmbeddingVectorSizeUsesServerConfig(t *testing.T) {
	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{QdrantVectorSize: 1024})
	handler := NewConfigHandler(appService, nil)
	if actual := handler.expectedEmbeddingVectorSize(); actual != 1024 {
		t.Fatalf("expected configured vector size 1024, got %d", actual)
	}
}

func TestEmbeddingModelResponseIncludesConfiguredVectorSize(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/embed" {
			http.NotFound(w, r)
			return
		}
		var probe struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&probe); err != nil || len(probe.Input) != 1 || probe.Input[0] != "LocalRAG model health probe" {
			http.Error(w, "expected model probe request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3,0.4]]}`))
	}))
	t.Cleanup(embeddingServer.Close)

	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{QdrantVectorSize: 4})
	handler := NewConfigHandler(appService, nil)
	payload, err := json.Marshal(TestEmbeddingModelRequest{
		Provider: "ollama",
		BaseURL:  embeddingServer.URL,
		Model:    "test-embedding",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/config/test-embedding-model", bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	handler.TestEmbeddingModel(context)

	var response TestModelResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusOK || !response.Success {
		t.Fatalf("expected successful embedding probe, status=%d response=%#v", recorder.Code, response)
	}
	if response.VectorSize != 4 || response.ExpectedVectorSize != 4 {
		t.Fatalf("expected actual and configured dimensions to be 4, got %#v", response)
	}
}

func TestFormatErrorMessageExplainsEmbeddingDimensionMigration(t *testing.T) {
	err := &service.EmbeddingDimensionMismatchError{BatchItem: 0, Expected: 768, Actual: 1024}
	message := formatErrorMessage(err)
	for _, expected := range []string{"1024", "QDRANT_VECTOR_SIZE=768", "QDRANT_COLLECTION_PREFIX", "重新索引"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected migration message to contain %q, got %q", expected, message)
		}
	}

	actual, configured := embeddingDimensionDetails(err)
	if actual != 1024 || configured != 768 {
		t.Fatalf("unexpected dimension details: actual=%d configured=%d", actual, configured)
	}
}

func TestReadinessReturnsReadyWithoutOptionalDependencies(t *testing.T) {
	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{})
	handler := NewConfigHandler(appService, nil)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.Readiness(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected readiness status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response ReadinessResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if response.Status != "ready" {
		t.Fatalf("expected ready status, got %#v", response)
	}
	if response.Checks["chat_model"].Status != "configured" {
		t.Fatalf("expected default chat model configuration to be reported, got %#v", response.Checks["chat_model"])
	}
}

func TestReadinessReturnsUnavailableWhenQdrantIsDown(t *testing.T) {
	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(qdrantServer.Close)

	serverConfig := model.ServerConfig{QdrantURL: qdrantServer.URL}
	qdrant := service.NewQdrantService(serverConfig)
	appService := service.NewAppService(qdrant, nil, nil, serverConfig)
	handler := NewConfigHandler(appService, qdrant)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.Readiness(context)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness status 503, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReadinessReturnsUnavailableWhenStagingManifestIsCorrupt(t *testing.T) {
	stagingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stagingDir, "manifest.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt staging manifest: %v", err)
	}
	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{StagingDir: stagingDir})
	handler := NewConfigHandler(appService, nil)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	handler.Readiness(context)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness status 503, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response ReadinessResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if response.Checks["upload_staging"].Status != "error" {
		t.Fatalf("expected staging readiness check to fail, got %#v", response.Checks["upload_staging"])
	}
	if strings.Contains(response.Checks["upload_staging"].ErrorMessage, stagingDir) {
		t.Fatalf("readiness error must not expose staging path: %#v", response.Checks["upload_staging"])
	}
}

func TestListModelsReturnsCandidateList(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2"},{"name":"qwen2.5"}]}`))
	}))
	t.Cleanup(upstream.Close)

	handler := NewConfigHandler(service.NewAppService(nil, nil, nil, model.ServerConfig{}), nil)
	recorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models", model.ModelListRequest{
		Type: model.ModelKindChat, Provider: "ollama", BaseURL: upstream.URL,
	}, handler.ListModels)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response model.ModelListResponse
	decodeJSONResponse(t, recorder.Body.Bytes(), &response)
	if !response.Success || len(response.Models) != 2 || response.Models[0].ID != "llama3.2" || response.Models[1].ID != "qwen2.5" {
		t.Fatalf("unexpected model list response: %#v", response)
	}
}

func TestProbeModelReturnsEmbeddingLatencyAndVectorFields(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/embed" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3,0.4]]}`))
	}))
	t.Cleanup(upstream.Close)

	handler := NewConfigHandler(service.NewAppService(nil, nil, nil, model.ServerConfig{QdrantVectorSize: 4}), nil)
	recorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models/probe", model.ModelProbeRequest{
		Type: model.ModelKindEmbedding, Provider: "ollama", BaseURL: upstream.URL, Model: "nomic-embed-text",
	}, handler.ProbeModel)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response model.ModelProbeResponse
	decodeJSONResponse(t, recorder.Body.Bytes(), &response)
	if !response.Success || response.LatencyMs < 0 || response.VectorSize != 4 || response.ExpectedVectorSize != 4 || response.DimensionMatch == nil || !*response.DimensionMatch {
		t.Fatalf("unexpected probe response: %#v", response)
	}
}

func TestProbeModelRejectsInvalidRequests(t *testing.T) {
	handler := NewConfigHandler(service.NewAppService(nil, nil, nil, model.ServerConfig{}), nil)
	for _, request := range []model.ModelProbeRequest{
		{Type: "invalid", Provider: "ollama", BaseURL: "http://127.0.0.1", Model: "model"},
		{Type: model.ModelKindChat, Provider: "ollama", Model: "model"},
		{Type: model.ModelKindChat, Provider: "ollama", BaseURL: "http://127.0.0.1"},
	} {
		recorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models/probe", request, handler.ProbeModel)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %#v, got %d: %s", request, recorder.Code, recorder.Body.String())
		}
	}
}

func TestProbeModelUsesMatchingStoredAPIKeyWithoutLeakingIt(t *testing.T) {
	const storedKey = "stored-key-must-not-leak"
	var upstreamAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthorization = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"stored-key-must-not-leak"}`))
	}))
	t.Cleanup(upstream.Close)

	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{})
	setModelConfigs(t, appService,
		model.ChatConfig{Provider: "ollama", BaseURL: "http://127.0.0.1:11434", Model: "chat-model"},
		model.EmbeddingConfig{Provider: "openai-compatible", BaseURL: upstream.URL + "/v1", Model: "embed-model", APIKey: storedKey},
	)
	handler := NewConfigHandler(appService, nil)
	recorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models/probe", model.ModelProbeRequest{
		Type: model.ModelKindEmbedding, Provider: "openai", BaseURL: upstream.URL, Model: "embed-model",
	}, handler.ProbeModel)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected safe upstream failure as 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if upstreamAuthorization != "Bearer "+storedKey {
		t.Fatalf("expected saved key upstream, got authorization %q", upstreamAuthorization)
	}
	var response model.ModelProbeResponse
	decodeJSONResponse(t, recorder.Body.Bytes(), &response)
	if response.Success || strings.Contains(recorder.Body.String(), storedKey) || strings.Contains(response.ErrorMessage, storedKey) {
		t.Fatalf("probe response leaked key or unexpectedly succeeded: %#v body=%s", response, recorder.Body.String())
	}
}

func TestProbeModelHonorsExplicitAPIKeyClear(t *testing.T) {
	const storedKey = "stored-key-must-not-be-used"
	var upstreamAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3,0.4]}]}`))
	}))
	t.Cleanup(upstream.Close)

	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{QdrantVectorSize: 4})
	setModelConfigs(t, appService,
		model.ChatConfig{Provider: "ollama", BaseURL: "http://127.0.0.1:11434", Model: "chat-model"},
		model.EmbeddingConfig{Provider: "openai-compatible", BaseURL: upstream.URL + "/v1", Model: "embed-model", APIKey: storedKey},
	)
	handler := NewConfigHandler(appService, nil)
	configured := true
	recorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models/probe", model.ModelProbeRequest{
		Type: model.ModelKindEmbedding, Provider: "openai", BaseURL: upstream.URL, Model: "embed-model",
		APIKeyConfigured: &configured, ClearAPIKey: true,
	}, handler.ProbeModel)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected safe probe response, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if upstreamAuthorization != "" {
		t.Fatalf("expected cleared key not to be sent, got authorization %q", upstreamAuthorization)
	}
	var response model.ModelProbeResponse
	decodeJSONResponse(t, recorder.Body.Bytes(), &response)
	if !response.Success {
		t.Fatalf("expected probe without stored key to succeed, got %#v", response)
	}
}

func TestProbeSaveAndRuntimeChatUseCanonicalOpenAIEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"model":"chat-model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
	}))
	t.Cleanup(upstream.Close)

	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{})
	handler := NewConfigHandler(appService, nil)
	probeRecorder := invokeConfigHandler(t, http.MethodPost, "/api/config/models/probe", model.ModelProbeRequest{
		Type: model.ModelKindChat, Provider: "openai", BaseURL: upstream.URL, Model: "chat-model",
	}, handler.ProbeModel)
	if probeRecorder.Code != http.StatusOK {
		t.Fatalf("expected probe status 200, got %d: %s", probeRecorder.Code, probeRecorder.Body.String())
	}
	var probe model.ModelProbeResponse
	decodeJSONResponse(t, probeRecorder.Body.Bytes(), &probe)
	if !probe.Success {
		t.Fatalf("expected probe success, got %#v", probe)
	}

	if _, err := appService.UpdateConfig(model.ConfigUpdateRequest{
		Chat:      model.ChatConfig{Provider: "openai", BaseURL: upstream.URL, Model: "chat-model"},
		Embedding: model.EmbeddingConfig{Provider: "ollama", BaseURL: "http://127.0.0.1:11434", Model: "embedding-model"},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	chatConfig := appService.CurrentChatConfig()
	if chatConfig.Provider != "openai-compatible" || chatConfig.BaseURL != upstream.URL+"/v1" {
		t.Fatalf("expected canonical saved chat config, got %#v", chatConfig)
	}

	response, err := service.NewLLMService().Chat(model.ChatCompletionRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "hello"}},
		Config:   chatConfig,
	})
	if err != nil || len(response.Choices) != 1 || response.Choices[0].Message.Content != "OK" {
		t.Fatalf("expected saved config to drive runtime chat, response=%#v err=%v", response, err)
	}
}

func TestHealthSummaryReportsProbeSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/chat":
			var request struct {
				Messages []model.ChatMessage `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) != 1 || request.Messages[0].Content != "Please reply with OK only." {
				http.Error(w, "expected model probe chat request", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"model":"chat-model","message":{"content":"OK"}}`))
		case "/api/embed":
			var request struct {
				Input []string `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Input) != 1 || request.Input[0] != "LocalRAG model health probe" {
				http.Error(w, "expected model probe embedding request", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3,0.4]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	appService := service.NewAppService(nil, nil, nil, model.ServerConfig{QdrantVectorSize: 4})
	setModelConfigs(t, appService,
		model.ChatConfig{Provider: "ollama", BaseURL: upstream.URL, Model: "chat-model"},
		model.EmbeddingConfig{Provider: "ollama", BaseURL: upstream.URL, Model: "embedding-model"},
	)
	handler := NewConfigHandler(appService, nil)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/config/health", nil)
	handler.HealthSummary(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var summary HealthSummaryResponse
	decodeJSONResponse(t, recorder.Body.Bytes(), &summary)
	if summary.ChatModel.Status != "ok" || summary.ChatModel.LatencyMs < 0 || summary.EmbeddingModel.Status != "ok" || summary.EmbeddingModel.LatencyMs < 0 {
		t.Fatalf("unexpected health summary: %#v", summary)
	}
}

func invokeConfigHandler(t *testing.T, method, path string, payload any, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	return recorder
}

func decodeJSONResponse(t *testing.T, body []byte, destination any) {
	t.Helper()
	if err := json.Unmarshal(body, destination); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, body)
	}
}

func setModelConfigs(t *testing.T, appService *service.AppService, chat model.ChatConfig, embedding model.EmbeddingConfig) {
	t.Helper()
	if _, err := appService.UpdateConfig(model.ConfigUpdateRequest{Chat: chat, Embedding: embedding}); err != nil {
		t.Fatalf("configure models: %v", err)
	}
}
