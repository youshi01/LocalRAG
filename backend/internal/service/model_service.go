package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"localrag/internal/model"
)

const (
	modelDiscoveryTimeout = 8 * time.Second
	modelProbeTimeout     = 15 * time.Second
)

type ModelService struct {
	client *http.Client
}

func NewModelService() *ModelService {
	return &ModelService{client: &http.Client{Timeout: modelDiscoveryTimeout}}
}

func (s *ModelService) ListModels(parent context.Context, request model.ModelListRequest) (model.ModelListResponse, error) {
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

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: modelDiscoveryTimeout}
	}
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

	var options []model.ModelOption
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

func decodeStrictJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
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

func (s *ModelService) Probe(_ context.Context, request model.ModelProbeRequest, _ int) (model.ModelProbeResponse, error) {
	if _, err := normalizeModelKind(request.Type); err != nil {
		return model.ModelProbeResponse{}, err
	}
	return model.ModelProbeResponse{}, nil
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
