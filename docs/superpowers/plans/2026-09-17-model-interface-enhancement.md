# LocalRAG Model Interface Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ] ) syntax for tracking.

**Goal:** Add unified model listing and real model probing for Ollama and OpenAI Compatible providers, expose latency and embedding-dimension health data, and wire the workflow into the LocalRAG settings page without breaking existing model-test endpoints.

**Architecture:** Add a backend 'ModelService' that owns provider URL normalization, /api/tags and /models discovery, and single-attempt Chat/Embedding probes with bounded timeouts. 'ConfigHandler' will use that service for the new endpoints, legacy test endpoints, and model health summary. The frontend will keep the existing editable model inputs and add discovery, candidate selection, probe status, latency, and vector-dimension feedback in 'ModelConfigTest'.

**Tech Stack:** Go 1.25, Gin, net/http, httptest; React 18, TypeScript, Vite, Vitest.

**Spec:** docs/superpowers/specs/2026-09-17-model-interface-design.md

## Global Constraints

- Support exactly the model kinds 'chat' and 'embedding'.
- Support Ollama native endpoints and OpenAI Compatible endpoints; accept the existing 'openai' provider alias and normalize it to 'openai-compatible' for compatibility.
- Discovery is candidate information; only a successful real probe proves model availability.
- Discovery timeout is at most 8 seconds; probe timeout is at most 15 seconds; discovery and probe do not retry.
- Never return or log API keys, Authorization headers, credential-bearing URLs, or raw upstream response bodies.
- Keep /api/config/test-chat-model and /api/config/test-embedding-model available with their existing response fields.
- /readyz must remain configuration-only and must not execute model inference.
- Do not modify the existing midterm DOCX or the untracked LocalRAG-midterm-source.zip.
- Every production change must have a failing test written and observed before implementation.

---

## File Map

### Backend

- Modify: backend/internal/model/types.go — add model discovery/probe request, option, and response contracts.
- Create: backend/internal/service/model_service.go — provider normalization, discovery, probe HTTP calls, timeout, latency, and safe error classification.
- Create: backend/internal/service/model_service_test.go — service-level red/green tests.
- Modify: backend/internal/handler/config_handler.go — inject ModelService, add new handlers, adapt legacy tests, and reuse probes for health summary.
- Modify: backend/internal/handler/config_handler_test.go — handler validation, API-key redaction, probe response, and health checks.
- Modify: backend/internal/router/router.go — register the two new protected configuration routes.
- Modify: backend/internal/router/router_e2e_test.go — add route coverage and fixture responses for /api/tags and /v1/models.

### Frontend

- Modify: frontend/src/services/api.ts — add model discovery/probe types and request functions.
- Modify: frontend/src/services/api.test.ts — verify request paths, methods, request bodies, and response shapes.
- Create: frontend/src/components/settings/modelOptions.ts — pure model-option reset/label helpers.
- Create: frontend/src/components/settings/modelOptions.test.ts — helper tests.
- Modify: frontend/src/components/settings/ModelConfigProbe.tsx — add discovery, candidate selection, probe calls, and reset behavior.
- Modify: frontend/src/components/settings/tabs/AISettings.tsx — pass model-change callbacks.
- Modify: frontend/src/styles/settings-panel.css — style discovery controls and status layout.

### Documentation

- Modify: README.md — document settings discovery, probe latency, and vector checks.
- Modify: docs/architecture.md — document ModelService and the two new routes.

---

## Task 1: Define model contracts and provider URL normalization

**Files:**
- Modify: backend/internal/model/types.go
- Create: backend/internal/service/model_service.go
- Create: backend/internal/service/model_service_test.go

**Interfaces:**
- Produces model.ModelKind, model.ModelListRequest, model.ModelOption, model.ModelListResponse, model.ModelProbeRequest, and model.ModelProbeResponse.
- Produces normalizeModelProvider(provider string) (string, error) and normalizeModelEndpoint(provider, baseURL string) (normalizedProvider, normalizedBaseURL string, err error) inside service.
- ModelService exposes NewModelService(), ListModels(context.Context, model.ModelListRequest) (model.ModelListResponse, error), and Probe(context.Context, model.ModelProbeRequest, int) (model.ModelProbeResponse, error).

- [ ] **Step 1: Write the failing contract and normalization tests**

Add to backend/internal/service/model_service_test.go:

~~~go
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
~~~

- [ ] **Step 2: Run the focused tests and verify the expected red failure**

Run from E:\codex\ai-localbase-main\ai-localbase-main\backend:

~~~powershell
go test ./internal/service -run 'TestNormalizeModelEndpoint|TestNormalizeModelProviderAcceptsLegacyOpenAIName' -count=1
~~~

Expected: FAIL because the model contracts and normalization functions do not exist yet.

- [ ] **Step 3: Add the model API contracts**

Add to backend/internal/model/types.go. Use the JSON names shown below; the Go declarations should include the corresponding tags such as json:"type":

~~~go
type ModelKind string

const (
    ModelKindChat      ModelKind = "chat"
    ModelKindEmbedding ModelKind = "embedding"
)

type ModelListRequest struct {
    Type     ModelKind
    Provider string
    BaseURL  string
    APIKey   string
}

type ModelOption struct {
    ID      string
    Name    string
    Type    ModelKind
    OwnedBy string
}

type ModelListResponse struct {
    Success      bool
    Provider     string
    Type         ModelKind
    Models       []ModelOption
    LatencyMs    int64
    ErrorCode    string
    ErrorMessage string
}

type ModelProbeRequest struct {
    Type        ModelKind
    Provider    string
    BaseURL     string
    Model       string
    APIKey      string
    Temperature float64
}

type ModelProbeResponse struct {
    Success            bool
    Type               ModelKind
    Provider           string
    Model              string
    LatencyMs          int64
    VectorSize         int
    ExpectedVectorSize int
    DimensionMatch     *bool
    ModelInfo          string
    ErrorCode          string
    ErrorMessage       string
}
~~~

- [ ] **Step 4: Add the minimal normalization implementation and service skeleton**

In backend/internal/service/model_service.go, add ModelService, NewModelService, constants modelDiscoveryTimeout = 8 * time.Second and modelProbeTimeout = 15 * time.Second, and these rules:

- Trim whitespace and trailing slash.
- Accept ollama, openai, and openai-compatible case-insensitively; normalize the latter two to openai-compatible.
- Parse the URL and require http or https plus a host.
- Reject a path ending in /models, /chat/completions, or /embeddings.
- For Ollama, remove a trailing /v1.
- For OpenAI Compatible, append /v1 unless the path already ends in /v1.

- [ ] **Step 5: Run the focused tests and verify green**

~~~powershell
go test ./internal/service -run 'TestNormalizeModelEndpoint|TestNormalizeModelProviderAcceptsLegacyOpenAIName' -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Commit the contracts and normalization**

~~~powershell
git add backend/internal/model/types.go backend/internal/service/model_service.go backend/internal/service/model_service_test.go
git commit -m "feat: add model capability contracts"
~~~

## Task 2: Implement model discovery for Ollama and OpenAI Compatible

**Files:**
- Modify: backend/internal/service/model_service.go
- Modify: backend/internal/service/model_service_test.go

**Interfaces:**
- ModelService.ListModels accepts a validated ModelListRequest and returns normalized provider/type, sorted options, latency, or a safe error code/message.
- Ollama uses GET <base>/api/tags; OpenAI Compatible uses GET <base>/models.

- [ ] **Step 1: Write failing discovery tests**

Add tests with httptest.NewServer and an injected server.Client():

~~~go
func TestListModelsReadsOllamaTagsAndMeasuresLatency(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet || r.URL.Path != "/api/tags" {
            t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = io.WriteString(w, "{\"models\":[{\"name\":\"nomic-embed-text\"},{\"model\":\"qwen3.5:9b\"}]}")
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
        _, _ = io.WriteString(w, "{\"data\":[{\"id\":\"chat-model\",\"owned_by\":\"test\"}]}")
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
        http.Error(w, "{\"error\":\"secret-discovery-key\"}", http.StatusUnauthorized)
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
~~~

- [ ] **Step 2: Run the discovery tests and verify red**

~~~powershell
go test ./internal/service -run 'TestListModels' -count=1
~~~

Expected: FAIL because ListModels has not been implemented.

- [ ] **Step 3: Implement one-shot discovery requests**

Implement ListModels so it:

1. Validates and normalizes provider and Base URL.
2. Creates a child context with an 8-second deadline unless the caller already has an earlier deadline.
3. Starts the latency timer immediately before client.Do and records elapsed milliseconds for both success and safe failure.
4. Sends Authorization: Bearer <apiKey> only for OpenAI Compatible when the trimmed key is non-empty.
5. Parses Ollama models[].name, falling back to models[].model.
6. Parses OpenAI data[].id and optional owned_by.
7. Deduplicates by model ID and sorts by ID.
8. Makes no retry attempt.
9. Maps HTTP 401/403 to authentication_failed, 404 to endpoint_not_found, context deadline to timeout, connection errors to provider_unreachable, invalid JSON to invalid_response, and other upstream statuses to upstream_error.
10. Returns only safe fixed messages such as 模型服务鉴权失败, 模型服务不可达, 模型列表响应格式无效, or 模型服务返回 HTTP <status>.

- [ ] **Step 4: Run discovery tests and the backend service package**

~~~powershell
go test ./internal/service -run 'TestNormalizeModel|TestListModels' -count=1
go test ./internal/service -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Commit discovery**

~~~powershell
git add backend/internal/service/model_service.go backend/internal/service/model_service_test.go
git commit -m "feat: discover provider models"
~~~

## Task 3: Implement single-attempt Chat and Embedding probes

**Files:**
- Modify: backend/internal/service/model_service.go
- Modify: backend/internal/service/model_service_test.go

**Interfaces:**
- ModelService.Probe(ctx, request, expectedVectorSize) returns latency even on upstream failure.
- Chat probes validate non-empty assistant content.
- Embedding probes validate a non-empty vector and return VectorSize, ExpectedVectorSize, and a non-nil DimensionMatch.

- [ ] **Step 1: Write failing probe tests**

Add tests for:

- Ollama Chat request path /api/chat, stream:false, requested model, and non-empty response.
- OpenAI Compatible Chat request path /v1/chat/completions, Bearer header, and non-empty choices content.
- Ollama Embedding request path /api/embed, dimension match, and returned latency.
- OpenAI Compatible Embedding request path /v1/embeddings, dimension mismatch, and error_code == dimension_mismatch.
- HTTP 400 model-not-found with exactly one request.
- Parent context deadline producing error_code == timeout without waiting for the 15-second default.

Use assertions like:

~~~go
if result.Success != expected {
    t.Fatalf("expected success=%v, got %#v", expected, result)
}
if result.LatencyMs < 0 {
    t.Fatalf("expected non-negative latency, got %d", result.LatencyMs)
}
if result.DimensionMatch == nil || *result.DimensionMatch != false {
    t.Fatalf("expected dimension mismatch marker, got %#v", result.DimensionMatch)
}
~~~

- [ ] **Step 2: Run the focused probe tests and verify red**

~~~powershell
go test ./internal/service -run 'TestProbe' -count=1
~~~

Expected: FAIL because the probe methods and provider payloads are not implemented.

- [ ] **Step 3: Implement Chat probe payloads and response validation**

Use one fixed message 'Please reply with OK only.' and temperature 0 unless the request provides a non-negative temperature. For Ollama send stream:false, think:false, and options.temperature; for OpenAI Compatible send stream:false, temperature:0, and max_tokens:16.

Accept only a successful HTTP status and non-empty content. Return the response model when provided, otherwise the requested model. Do not call LLMService.Chat, because it has a longer runtime timeout and retry policy that would distort health latency.

- [ ] **Step 4: Implement Embedding probe payloads and dimension validation**

Use one fixed short input 'LocalRAG model health probe'. Parse Ollama embeddings and OpenAI data[].embedding; reject empty arrays and empty vectors. If expectedVectorSize > 0, compare it with the first vector length and return dimension_mismatch with Success=false when they differ. If expected size is zero, mark DimensionMatch=true after vector validation.

- [ ] **Step 5: Add bounded timeout and safe error classification**

Wrap the operation in a child context capped at 15 seconds. Use the Ollama runtime scheduler for Ollama probe calls so health checks do not bypass existing single-flight protection; do not retry the HTTP request. Map model-not-found responses to model_not_found, 401/403 to authentication_failed, context deadline to timeout, empty content/vector to empty_response, invalid JSON to invalid_response, and other statuses to upstream_error.

- [ ] **Step 6: Run probe and package tests**

~~~powershell
go test ./internal/service -run 'TestNormalizeModel|TestListModels|TestProbe' -count=1
go test ./internal/service -count=1
~~~

Expected: PASS.

- [ ] **Step 7: Commit probes**

~~~powershell
git add backend/internal/service/model_service.go backend/internal/service/model_service_test.go
git commit -m "feat: probe chat and embedding models"
~~~

## Task 4: Integrate handlers, legacy endpoints, and health summary

**Files:**
- Modify: backend/internal/handler/config_handler.go
- Modify: backend/internal/handler/config_handler_test.go

**Interfaces:**
- ConfigHandler owns a *service.ModelService initialized by NewConfigHandler.
- Add ListModels(*gin.Context) at /api/config/models.
- Add ProbeModel(*gin.Context) at /api/config/models/probe.
- Existing TestChatModel and TestEmbeddingModel call the same ModelService.Probe and continue returning TestModelResponse.
- checkChatModelHealth and checkEmbeddingModelHealth use the same probe service; Readiness remains unchanged.

- [ ] **Step 1: Write failing handler tests**

Add tests that create an httptest.Server, configure AppService to point at it, invoke Gin handlers, and assert:

1. ListModels returns the candidate list and HTTP 200.
2. ProbeModel returns latency and vector fields for an embedding response.
3. Invalid type, empty Base URL, and empty model return HTTP 400.
4. When the request omits apiKey but provider and Base URL match saved config, the stored key is used upstream while neither response body nor error contains that key.
5. Legacy TestEmbeddingModel still returns TestModelResponse with vector_size and expected_vector_size.
6. Health summary reports ok and non-negative latency when both probes succeed.

Example assertion:

~~~go
if recorder.Code != http.StatusOK {
    t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
}
var response model.ModelProbeResponse
decodeJSONResponse(t, recorder.Body.Bytes(), &response)
if !response.Success || response.LatencyMs < 0 {
    t.Fatalf("unexpected probe response: %#v", response)
}
~~~

- [ ] **Step 2: Run handler tests and verify red**

~~~powershell
go test ./internal/handler -run 'Test(ListModels|ProbeModel|EmbeddingModel|HealthSummary)' -count=1
~~~

Expected: FAIL because the handlers still use only the old service paths and the new methods do not exist.

- [ ] **Step 3: Inject and use ModelService**

Add modelService *service.ModelService to ConfigHandler and initialize it in NewConfigHandler with service.NewModelService().

Add:

~~~go
func (h *ConfigHandler) resolveModelAPIKey(candidate string, kind model.ModelKind, provider, baseURL string) string
~~~

It returns the candidate key when non-empty; otherwise it may use the saved Chat or Embedding key only when provider and normalized endpoint match the corresponding saved config.

Implement ListModels and ProbeModel with ShouldBindJSON, call the resolver, return HTTP 400 for validation errors, and return the safe service response with HTTP 200 for completed upstream failures.

- [ ] **Step 4: Adapt legacy endpoints without changing their JSON contract**

Convert TestChatModelRequest and TestEmbeddingModelRequest to ModelProbeRequest, call Probe, then map success, latency_ms, error_message, vector_size, expected_vector_size, and model_info to the existing TestModelResponse. Keep existing formatErrorMessage behavior for older internal errors and use safe probe messages for new errors.

- [ ] **Step 5: Reuse probes in health summary**

Replace the direct NewLLMService().Chat and NewRagService().EmbedTexts calls in checkChatModelHealth and checkEmbeddingModelHealth with ModelService.Probe. Preserve existing ComponentHealth JSON fields and use the probe error message. Do not call new probe methods from Readiness.

- [ ] **Step 6: Run handler tests and verify green**

~~~powershell
go test ./internal/handler -run 'Test(ListModels|ProbeModel|EmbeddingModel|HealthSummary|Readiness)' -count=1
go test ./internal/handler -count=1
~~~

Expected: PASS.

- [ ] **Step 7: Commit handler integration**

~~~powershell
git add backend/internal/handler/config_handler.go backend/internal/handler/config_handler_test.go
git commit -m "feat: expose model discovery and probing handlers"
~~~

## Task 5: Register and verify router endpoints

**Files:**
- Modify: backend/internal/router/router.go
- Modify: backend/internal/router/router_e2e_test.go

- [ ] **Step 1: Write failing route tests**

Add TestRouterModelDiscoveryAndProbeEndpoints using newTestRouter(t):

- POST /api/config/models with the configured Ollama endpoint and assert HTTP 200, success=true, and model options.
- POST /api/config/models/probe with the configured Chat model and assert HTTP 200, success=true, and latency_ms present.
- POST the same endpoints with empty required fields and assert HTTP 400.

- [ ] **Step 2: Run the route tests and verify red**

~~~powershell
go test ./internal/router -run 'TestRouterModelDiscoveryAndProbeEndpoints' -count=1
~~~

Expected: FAIL with route-not-found or an empty fixture response.

- [ ] **Step 3: Register the routes**

Inside the authenticated api group in backend/internal/router/router.go, add immediately before the legacy test routes:

~~~go
api.POST("/config/models", configHandler.ListModels)
api.POST("/config/models/probe", configHandler.ProbeModel)
api.POST("/config/test-chat-model", configHandler.TestChatModel)
api.POST("/config/test-embedding-model", configHandler.TestEmbeddingModel)
~~~

- [ ] **Step 4: Extend the model test fixture**

In handleModelAPI, add GET /api/tags returning models with qwen3.5:9b and nomic-embed-text, and GET /v1/models returning data with chat-test-model and embedding-test-model. Keep all existing Chat and Embedding paths unchanged.

- [ ] **Step 5: Run router and backend regression tests**

~~~powershell
go test ./internal/router -run 'TestRouterModelDiscoveryAndProbeEndpoints|TestRouterConfigEndpoints|TestRouter.*Chat|TestRouter.*Embedding' -count=1
go test ./... -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Commit routes and fixture coverage**

~~~powershell
git add backend/internal/router/router.go backend/internal/router/router_e2e_test.go
git commit -m "feat: route model discovery and probes"
~~~

## Task 6: Add frontend API contracts and request tests

**Files:**
- Modify: frontend/src/services/api.ts
- Modify: frontend/src/services/api.test.ts

**Interfaces:**
- Add ModelKind = 'chat' | 'embedding'.
- Add ModelOption, ModelListResponse, and ModelProbeResponse matching backend snake_case response fields.
- Add fetchAvailableModels(config, type) and probeModel(config, type).

- [ ] **Step 1: Write failing API tests**

Use Vitest and vi.stubGlobal('fetch', vi.fn()) to verify:

~~~typescript
it('posts the draft endpoint to fetch available models', async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
    success: true,
    provider: 'ollama',
    type: 'chat',
    models: [{ id: 'qwen3.5:9b', name: 'qwen3.5:9b', type: 'chat' }],
    latency_ms: 24,
  }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetchMock)

  const response = await fetchAvailableModels({
    provider: 'ollama', baseUrl: 'http://localhost:11434', apiKey: '',
  }, 'chat')

  expect(fetchMock).toHaveBeenCalledWith('/api/config/models', expect.objectContaining({ method: 'POST' }))
  expect(response.models[0].id).toBe('qwen3.5:9b')
})

it('posts the selected model to probe it', async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
    success: true, type: 'embedding', provider: 'ollama', model: 'nomic-embed-text',
    latency_ms: 214, vector_size: 768, expected_vector_size: 768,
    dimension_match: true,
  }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetchMock)

  const response = await probeModel({
    provider: 'ollama', baseUrl: 'http://localhost:11434', model: 'nomic-embed-text', apiKey: '',
  }, 'embedding')

  expect(fetchMock).toHaveBeenCalledWith('/api/config/models/probe', expect.objectContaining({ method: 'POST' }))
  expect(response.dimension_match).toBe(true)
})
~~~

Restore original fetch in afterEach so existing API tests remain isolated.

- [ ] **Step 2: Run focused API tests and verify red**

~~~powershell
npm test -- --run src/services/api.test.ts
~~~

Expected: FAIL because the new exported functions and types do not exist.

- [ ] **Step 3: Implement frontend types and request functions**

Add types near TestModelResponse and implement functions near testChatModelConfig:

~~~typescript
export const fetchAvailableModels = async (
  config: Pick<ChatConfig, 'provider' | 'baseUrl' | 'apiKey'> | Pick<EmbeddingConfig, 'provider' | 'baseUrl' | 'apiKey'>,
  type: ModelKind,
): Promise<ModelListResponse> => requestJson<ModelListResponse>(
  '/api/config/models',
  jsonRequest({ type, provider: config.provider, baseUrl: config.baseUrl, apiKey: config.apiKey }, { method: 'POST' }),
)

export const probeModel = async (
  config: ChatConfig | EmbeddingConfig,
  type: ModelKind,
): Promise<ModelProbeResponse> => requestJson<ModelProbeResponse>(
  '/api/config/models/probe',
  jsonRequest({
    type,
    provider: config.provider,
    baseUrl: config.baseUrl,
    model: config.model,
    apiKey: config.apiKey,
    ...(type === 'chat' ? { temperature: config.temperature } : {}),
  }, { method: 'POST' }),
)
~~~

Do not send apiKeyConfigured, clearApiKey, or any other UI-only fields.

- [ ] **Step 4: Run API tests and all frontend unit tests**

~~~powershell
npm test -- --run src/services/api.test.ts
npm test
~~~

Expected: PASS.

- [ ] **Step 5: Commit frontend API integration**

~~~powershell
git add frontend/src/services/api.ts frontend/src/services/api.test.ts
git commit -m "feat: add frontend model discovery API"
~~~

## Task 7: Add settings-page discovery, selection, and probe feedback

**Files:**
- Modify: frontend/src/components/settings/ModelConfigProbe.tsx
- Modify: frontend/src/components/settings/tabs/AISettings.tsx
- Modify: frontend/src/styles/settings-panel.css
- Create: frontend/src/components/settings/modelOptions.ts
- Create: frontend/src/components/settings/modelOptions.test.ts

- [ ] **Step 1: Add a UI-state testable helper before editing the component**

Create modelOptions.ts with:

~~~typescript
import type { ModelOption } from '../../services/api'

export const resetModelOptionsKey = (type: string, provider: string, baseUrl: string) =>
  type + ':' + provider.trim().toLowerCase() + ':' + baseUrl.trim().replace(/\/+$/, '')

export const modelOptionLabel = (option: ModelOption) =>
  option.owned_by ? option.name + ' · ' + option.owned_by : option.name
~~~

Create modelOptions.test.ts asserting that trailing-slash/endpoint changes produce a new key and that owner labels are rendered only when present. Run it first and observe the expected missing-module failure, then add the helper and rerun to green.

- [ ] **Step 2: Extend ModelConfigTest props and state**

Add onModelChange: (value: string) => void, import useEffect, fetchAvailableModels, probeModel, and the new helper. Maintain separate state for availableModels, loadingModels, modelListError, testing, result, and errorMessage.

When resetModelOptionsKey(props.type, props.provider, props.baseUrl) changes, clear candidates and discovery errors. This ensures a model from an old endpoint cannot remain selected after the endpoint changes.

- [ ] **Step 3: Add discovery control and candidate select**

Render an 获取模型 button beside the existing probe button. Disable it while loading or when Base URL is blank. On success, store response.models; on success=false, show error_message without replacing the manually entered model.

When options exist, render a select with aria-label='可用模型' and an empty 请选择候选模型 option. Selecting an item invokes onModelChange; the existing text input in AISettings remains the manual fallback.

- [ ] **Step 4: Route probe through the new endpoint and retain latency details**

Change handleTest to call probeModel with the current draft config. Keep the existing success/error result layout and add error-code-independent display of latency_ms, model_info, vector_size, and expected_vector_size. Use 探测模型 as the button label and keep the existing required-field guard.

- [ ] **Step 5: Pass model-change callbacks from AISettings**

Add to the Chat probe:

~~~tsx
onModelChange={(value) => onChatConfigChange('model', value)}
~~~

Add to the Embedding probe:

~~~tsx
onModelChange={(value) => onEmbeddingConfigChange('model', value)}
~~~

Keep all API key inputs and save/discard flow unchanged.

- [ ] **Step 6: Add focused CSS and run frontend checks**

Add styles for .model-discovery-controls, .model-discovery-select, and .model-discovery-error in the existing settings stylesheet. Reuse existing button variables and add a mobile rule under the current settings media query so controls stack at narrow widths.

Run:

~~~powershell
npm test -- --run src/components/settings/modelOptions.test.ts
npm test
npm run typecheck
npm run lint
npm run build
~~~

Expected: PASS with no TypeScript or lint errors and a successful production build.

- [ ] **Step 7: Commit the settings UI**

~~~powershell
git add frontend/src/components/settings/ModelConfigProbe.tsx frontend/src/components/settings/modelOptions.ts frontend/src/components/settings/modelOptions.test.ts frontend/src/components/settings/tabs/AISettings.tsx frontend/src/styles/settings-panel.css
git commit -m "feat: add model discovery controls"
~~~

## Task 8: Update user-facing documentation and run the complete verification matrix

**Files:**
- Modify: README.md
- Modify: docs/architecture.md

- [ ] **Step 1: Document the settings workflow**

Update the model configuration section to state:

- Ollama Base URL is the service root such as http://localhost:11434; Docker uses http://host.docker.internal:11434.
- OpenAI Compatible Base URL may be the provider root or /v1; the backend normalizes it.
- 获取模型 reads candidates from the provider but does not download models.
- 探测模型 performs a real request and displays latency.
- Embedding success requires the returned dimension to match QDRANT_VECTOR_SIZE.
- A probe failure does not automatically overwrite or save the current model configuration.

Add the two routes to the API table in docs/architecture.md and list model_service.go in the backend service tree.

- [ ] **Step 2: Run formatting and static checks**

~~~powershell
gofmt -w backend/internal/model/types.go backend/internal/service/model_service.go backend/internal/service/model_service_test.go backend/internal/handler/config_handler.go backend/internal/handler/config_handler_test.go backend/internal/router/router.go backend/internal/router/router_e2e_test.go
go test ./... -count=1
npm test
npm run typecheck
npm run lint
npm run build
~~~

Expected: all commands exit 0. If a test fails, add or update a regression test before changing production code.

- [ ] **Step 3: Run a real Ollama regression when the local service is available**

Verify:

1. POST /api/config/models lists locally installed Chat and Embedding candidates.
2. POST /api/config/models/probe reports non-negative latency for qwen3.5:9b.
3. Embedding probe reports the actual dimension for nomic-embed-text and compares it to the configured Qdrant dimension.
4. An invalid model returns success=false with a safe error and no credential text.

Do not start Docker or download models as part of this code change; the user will compile and run containers later.

- [ ] **Step 4: Inspect the final diff and preserve unrelated files**

~~~powershell
git status --short
git diff --stat HEAD~8..HEAD
git diff --check HEAD~8..HEAD
~~~

Confirm the previously modified midterm DOCX and untracked source ZIP were not staged or altered.

- [ ] **Step 5: Commit documentation and final verification record**

~~~powershell
git add README.md docs/architecture.md
git commit -m "docs: describe model discovery and health probes"
~~~

Report exact test/build outputs, new endpoint paths, provider coverage, and whether live Ollama regression was available.
