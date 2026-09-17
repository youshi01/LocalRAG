package service

import (
	"context"
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
