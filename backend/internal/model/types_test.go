package model

import (
	"encoding/json"
	"testing"
)

func TestModelListResponseJSONUsesSnakeCaseWireNames(t *testing.T) {
	response := ModelListResponse{
		Success:  true,
		Provider: "openai",
		Type:     ModelKindChat,
		Models: []ModelOption{{
			ID:      "gpt-test",
			Name:    "GPT Test",
			Type:    ModelKindChat,
			OwnedBy: "test-owner",
		}},
		LatencyMs:    12,
		ErrorCode:    "",
		ErrorMessage: "",
	}

	assertJSONKeys(t, response, []string{
		"success", "provider", "type", "models", "latency_ms", "error_code", "error_message",
	})
	assertNestedJSONKeys(t, response, "models", []string{"id", "name", "type", "owned_by"})
}

func TestModelProbeResponseJSONUsesSnakeCaseWireNames(t *testing.T) {
	dimensionMatch := true
	response := ModelProbeResponse{
		Success:            true,
		Type:               ModelKindEmbedding,
		Provider:           "openai",
		Model:              "embedding-test",
		LatencyMs:          21,
		VectorSize:         1536,
		ExpectedVectorSize: 1536,
		DimensionMatch:     &dimensionMatch,
		ModelInfo:          "test model",
		ErrorCode:          "",
		ErrorMessage:       "",
	}

	assertJSONKeys(t, response, []string{
		"success", "type", "provider", "model", "latency_ms", "vector_size",
		"expected_vector_size", "dimension_match", "model_info", "error_code", "error_message",
	})
}

func assertJSONKeys(t *testing.T, value any, expectedKeys []string) map[string]any {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, key := range expectedKeys {
		if _, ok := object[key]; !ok {
			t.Errorf("expected JSON key %q in %s", key, payload)
		}
	}

	return object
}

func assertNestedJSONKeys(t *testing.T, value any, parentKey string, expectedKeys []string) {
	t.Helper()

	object := assertJSONKeys(t, value, []string{parentKey})
	items, ok := object[parentKey].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected non-empty JSON array at %q in %#v", parentKey, object[parentKey])
	}

	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object in %q, got %#v", parentKey, items[0])
	}
	for _, key := range expectedKeys {
		if _, ok := item[key]; !ok {
			t.Errorf("expected nested JSON key %q in %s", key, object[parentKey])
		}
	}
}
