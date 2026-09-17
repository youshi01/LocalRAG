package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"localrag/internal/model"
)

const (
	modelDiscoveryTimeout      = 8 * time.Second
	modelDiscoveryMaxBodyBytes = 4 << 20
	modelProbeTimeout          = 15 * time.Second
)

type ModelService struct {
	client *http.Client
}

func NewModelService() *ModelService {
	return &ModelService{client: &http.Client{}}
}

func (s *ModelService) ListModels(parent context.Context, request model.ModelListRequest) (model.ModelListResponse, error) {
	if parent == nil {
		parent = context.Background()
	}
	normalizedType, err := normalizeModelKind(request.Type)
	if err != nil {
		return model.ModelListResponse{}, err
	}
	normalizedProvider, baseURL, err := normalizeModelEndpoint(request.Provider, request.BaseURL)
	if err != nil {
		return model.ModelListResponse{}, err
	}
	result := model.ModelListResponse{Provider: normalizedProvider, Type: normalizedType}
	ctx, cancel := context.WithTimeout(parent, modelDiscoveryTimeout)
	defer cancel()

	path := "/api/tags"
	if normalizedProvider == "openai-compatible" {
		path = "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return model.ModelListResponse{}, err
	}
	if normalizedProvider == "openai-compatible" && strings.TrimSpace(request.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(request.APIKey))
	}

	client := singleAttemptHTTPClient(s.client)
	started := time.Now()
	response, err := client.Do(req)
	result.LatencyMs = nonNegativeMilliseconds(time.Since(started))
	if err != nil {
		result.ErrorCode, result.ErrorMessage = classifyDiscoveryTransportError(ctx, err)
		return result, nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		result.ErrorCode, result.ErrorMessage = classifyDiscoveryStatus(response.StatusCode)
		return result, nil
	}

	options := make([]model.ModelOption, 0)
	if normalizedProvider == "ollama" {
		var payload struct {
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
		}
		if err := decodeStrictJSON(response.Body, &payload); err != nil {
			result.ErrorCode, result.ErrorMessage = "invalid_response", "模型列表响应格式无效"
			return result, nil
		}
		seen := make(map[string]struct{}, len(payload.Models))
		for _, item := range payload.Models {
			id := strings.TrimSpace(item.Name)
			if id == "" {
				id = strings.TrimSpace(item.Model)
			}
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			options = append(options, model.ModelOption{ID: id, Name: id, Type: normalizedType})
		}
	} else {
		var payload struct {
			Data []struct {
				ID      string `json:"id"`
				OwnedBy string `json:"owned_by"`
			} `json:"data"`
		}
		if err := decodeStrictJSON(response.Body, &payload); err != nil {
			result.ErrorCode, result.ErrorMessage = "invalid_response", "模型列表响应格式无效"
			return result, nil
		}
		seen := make(map[string]struct{}, len(payload.Data))
		for _, item := range payload.Data {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			options = append(options, model.ModelOption{ID: id, Name: id, Type: normalizedType, OwnedBy: strings.TrimSpace(item.OwnedBy)})
		}
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	result.Success = true
	result.Models = options
	return result, nil
}

func singleAttemptHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}
func decodeStrictJSON(reader io.Reader, destination any) error {
	body, err := io.ReadAll(io.LimitReader(reader, modelDiscoveryMaxBodyBytes+1))
	if err != nil {
		return err
	}
	if len(body) > modelDiscoveryMaxBodyBytes {
		return errors.New("model discovery response exceeds size limit")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func nonNegativeMilliseconds(duration time.Duration) int64 {
	if duration < 0 {
		return 0
	}
	return duration.Milliseconds()
}

func classifyDiscoveryTransportError(ctx context.Context, err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout", "模型服务请求超时"
	}
	return "provider_unreachable", "模型服务不可达"
}

func classifyDiscoveryStatus(status int) (string, string) {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_failed", "模型服务鉴权失败"
	case http.StatusNotFound:
		return "endpoint_not_found", "模型服务端点不存在"
	default:
		return "upstream_error", fmt.Sprintf("模型服务返回 HTTP %d", status)
	}
}

type modelProbeError struct {
	code    string
	message string
}

func (e *modelProbeError) Error() string { return e.message }

func probeError(code, message string) error {
	return &modelProbeError{code: code, message: message}
}

func (s *ModelService) Probe(parent context.Context, request model.ModelProbeRequest, expectedVectorSize int) (model.ModelProbeResponse, error) {
	normalizedType, err := normalizeModelKind(request.Type)
	if err != nil {
		return model.ModelProbeResponse{}, err
	}
	normalizedProvider, baseURL, err := normalizeModelEndpoint(request.Provider, request.BaseURL)
	if err != nil {
		return model.ModelProbeResponse{}, err
	}
	requestedModel := strings.TrimSpace(request.Model)
	if requestedModel == "" {
		return model.ModelProbeResponse{}, fmt.Errorf("model name is required")
	}
	if parent == nil {
		parent = context.Background()
	}

	result := model.ModelProbeResponse{
		Type:               normalizedType,
		Provider:           normalizedProvider,
		Model:              requestedModel,
		ExpectedVectorSize: expectedVectorSize,
	}
	ctx, cancel := context.WithTimeout(parent, modelProbeTimeout)
	defer cancel()

	temperature := 0.0
	if request.Temperature >= 0 && !math.IsInf(request.Temperature, 0) && !math.IsNaN(request.Temperature) {
		temperature = request.Temperature
	}
	started := time.Now()
	var returnedModel string
	var vectorSize int
	call := func(runCtx context.Context) error {
		switch normalizedType {
		case model.ModelKindChat:
			var modelName string
			if normalizedProvider == "ollama" {
				modelName, err = s.probeOllamaChat(runCtx, baseURL, requestedModel, temperature)
			} else {
				modelName, err = s.probeOpenAIChat(runCtx, baseURL, requestedModel, strings.TrimSpace(request.APIKey), temperature)
			}
			returnedModel = modelName
			return err
		case model.ModelKindEmbedding:
			if normalizedProvider == "ollama" {
				vectorSize, err = s.probeOllamaEmbedding(runCtx, baseURL, requestedModel)
			} else {
				vectorSize, err = s.probeOpenAIEmbedding(runCtx, baseURL, requestedModel, strings.TrimSpace(request.APIKey))
			}
			return err
		default:
			return probeError("upstream_error", "模型服务请求失败")
		}
	}
	if normalizedProvider == "ollama" {
		if normalizedType == model.ModelKindChat {
			err = sharedModelRuntimeScheduler.run(ctx, modelRuntimePriorityLow, call)
		} else {
			err = sharedEmbeddingRuntimeScheduler.run(ctx, modelRuntimePriorityLow, call)
		}
	} else {
		err = call(ctx)
	}
	result.LatencyMs = nonNegativeMilliseconds(time.Since(started))
	if err != nil {
		result.ErrorCode, result.ErrorMessage = classifyProbeError(ctx, err)
		return result, nil
	}

	if normalizedType == model.ModelKindEmbedding {
		result.VectorSize = vectorSize
		matched := expectedVectorSize <= 0 || vectorSize == expectedVectorSize
		result.DimensionMatch = &matched
		if !matched {
			result.ErrorCode = "dimension_mismatch"
			result.ErrorMessage = "模型向量维度不匹配"
			return result, nil
		}
	} else if strings.TrimSpace(returnedModel) != "" {
		result.Model = strings.TrimSpace(returnedModel)
	}
	result.Success = true
	return result, nil
}

func (s *ModelService) probeOllamaChat(ctx context.Context, baseURL, modelName string, temperature float64) (string, error) {
	think := false
	var response ollamaChatResponse
	err := s.doProbeJSON(ctx, baseURL+"/api/chat", ollamaChatRequest{
		Model: modelName,
		Messages: []model.ChatMessage{{
			Role:    "user",
			Content: "Please reply with OK only.",
		}},
		Stream:  false,
		Think:   &think,
		Options: &ollamaOptions{Temperature: temperature},
	}, "", &response)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Message.Content) == "" {
		return "", probeError("empty_response", "模型服务返回为空")
	}
	return strings.TrimSpace(response.Model), nil
}

func (s *ModelService) probeOpenAIChat(ctx context.Context, baseURL, modelName, apiKey string, temperature float64) (string, error) {
	var response openAIChatResponse
	payload := struct {
		Model       string              `json:"model"`
		Messages    []model.ChatMessage `json:"messages"`
		Stream      bool                `json:"stream"`
		Temperature float64             `json:"temperature"`
		MaxTokens   int                 `json:"max_tokens"`
	}{
		Model:       modelName,
		Messages:    []model.ChatMessage{{Role: "user", Content: "Please reply with OK only."}},
		Stream:      false,
		Temperature: temperature,
		MaxTokens:   16,
	}
	if err := s.doProbeJSON(ctx, baseURL+"/chat/completions", payload, apiKey, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", probeError("empty_response", "模型服务返回为空")
	}
	return strings.TrimSpace(response.Model), nil
}

func (s *ModelService) probeOllamaEmbedding(ctx context.Context, baseURL, modelName string) (int, error) {
	var response ollamaEmbedResponse
	if err := s.doProbeJSON(ctx, baseURL+"/api/embed", ollamaEmbedRequest{
		Model: modelName,
		Input: []string{"LocalRAG model health probe"},
	}, "", &response); err != nil {
		return 0, err
	}
	if len(response.Embeddings) == 0 {
		return 0, probeError("empty_response", "模型服务返回为空")
	}
	for _, vector := range response.Embeddings {
		if len(vector) == 0 {
			return 0, probeError("empty_response", "模型服务返回为空")
		}
	}
	return len(response.Embeddings[0]), nil
}

func (s *ModelService) probeOpenAIEmbedding(ctx context.Context, baseURL, modelName, apiKey string) (int, error) {
	var response openAIEmbeddingResponse
	if err := s.doProbeJSON(ctx, baseURL+"/embeddings", openAIEmbeddingRequest{
		Model: modelName,
		Input: []string{"LocalRAG model health probe"},
	}, apiKey, &response); err != nil {
		return 0, err
	}
	if len(response.Data) == 0 {
		return 0, probeError("empty_response", "模型服务返回为空")
	}
	for _, item := range response.Data {
		if len(item.Embedding) == 0 {
			return 0, probeError("empty_response", "模型服务返回为空")
		}
	}
	return len(response.Data[0].Embedding), nil
}

func (s *ModelService) doProbeJSON(ctx context.Context, endpoint string, payload any, apiKey string, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return probeError("upstream_error", "模型服务请求失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return probeError("upstream_error", "模型服务请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := singleAttemptHTTPClient(s.client)
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return classifyProbeStatus(response.StatusCode, readProbeErrorText(response.Body))
	}
	if err := decodeStrictJSON(response.Body, destination); err != nil {
		return probeError("invalid_response", "模型服务响应格式无效")
	}
	return nil
}

func readProbeErrorText(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, modelDiscoveryMaxBodyBytes+1))
	if err != nil || len(body) > modelDiscoveryMaxBodyBytes {
		return ""
	}
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	if len(payload.Error) > 0 {
		var text string
		if json.Unmarshal(payload.Error, &text) == nil {
			return text
		}
		var details struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(payload.Error, &details) == nil {
			return details.Message
		}
	}
	return payload.Message
}

func classifyProbeStatus(status int, responseText string) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return probeError("authentication_failed", "模型服务鉴权失败")
	}
	text := strings.ToLower(responseText)
	if strings.Contains(text, "model") && (strings.Contains(text, "not found") || strings.Contains(text, "does not exist")) {
		return probeError("model_not_found", "指定模型不存在")
	}
	return probeError("upstream_error", "模型服务请求失败")
}

func classifyProbeError(ctx context.Context, err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout", "模型服务请求超时"
	}
	var probeErr *modelProbeError
	if errors.As(err, &probeErr) {
		return probeErr.code, probeErr.message
	}
	return "upstream_error", "模型服务请求失败"
}

func normalizeModelKind(kind model.ModelKind) (model.ModelKind, error) {
	switch kind {
	case model.ModelKindChat, model.ModelKindEmbedding:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported model type")
	}
}

func normalizeModelProvider(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return "ollama", nil
	case "openai", "openai-compatible":
		return "openai-compatible", nil
	default:
		return "", fmt.Errorf("unsupported model provider")
	}
}

func normalizeModelEndpoint(provider, baseURL string) (normalizedProvider, normalizedBaseURL string, err error) {
	normalizedProvider, err = normalizeModelProvider(provider)
	if err != nil {
		return "", "", err
	}

	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", "", fmt.Errorf("invalid model endpoint URL")
	}
	if parsed.User != nil {
		return "", "", fmt.Errorf("invalid model endpoint URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("invalid model endpoint URL")
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", fmt.Errorf("invalid model endpoint URL")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(parsed.Path)
	for _, suffix := range []string{"/models", "/chat/completions", "/embeddings"} {
		if strings.HasSuffix(lowerPath, suffix) {
			return "", "", fmt.Errorf("model endpoint URL must be a base URL")
		}
	}

	if normalizedProvider == "ollama" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/v1")
	} else if !strings.HasSuffix(strings.ToLower(parsed.Path), "/v1") {
		parsed.Path += "/v1"
	}

	return normalizedProvider, strings.TrimRight(parsed.String(), "/"), nil
}

// NormalizeModelEndpoint exposes the shared endpoint normalization contract to
// handlers without duplicating the normalization rules.
func NormalizeModelEndpoint(provider, baseURL string) (string, string, error) {
	return normalizeModelEndpoint(provider, baseURL)
}
