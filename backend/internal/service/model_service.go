package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"localrag/internal/model"
)

const (
	modelDiscoveryTimeout = 8 * time.Second
	modelProbeTimeout     = 15 * time.Second
)

type ModelService struct{}

func NewModelService() *ModelService {
	return &ModelService{}
}

func (s *ModelService) ListModels(_ context.Context, request model.ModelListRequest) (model.ModelListResponse, error) {
	if _, err := normalizeModelKind(request.Type); err != nil {
		return model.ModelListResponse{}, err
	}
	return model.ModelListResponse{}, nil
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
