package service

import (
	"testing"

	"localrag/internal/model"
)

func TestEmbeddingFingerprintNormalizesEquivalentEndpoints(t *testing.T) {
	first := embeddingFingerprintForConfig(model.EmbeddingConfig{
		Provider: "openai",
		BaseURL:  "http://127.0.0.1:9000",
		Model:    "embed-v1",
	}, 768)
	second := embeddingFingerprintForConfig(model.EmbeddingConfig{
		Provider: "openai-compatible",
		BaseURL:  "http://127.0.0.1:9000/v1/",
		Model:    "embed-v1",
	}, 768)
	if first == "" || first != second {
		t.Fatalf("expected equivalent embedding endpoints to share fingerprint: %q vs %q", first, second)
	}
}

func TestEmbeddingFingerprintChangesWhenModelOrDimensionChanges(t *testing.T) {
	base := model.EmbeddingConfig{Provider: "ollama", BaseURL: "http://localhost:11434", Model: "embed-a"}
	first := embeddingFingerprintForConfig(base, 768)
	if first == embeddingFingerprintForConfig(model.EmbeddingConfig{Provider: "ollama", BaseURL: base.BaseURL, Model: "embed-b"}, 768) {
		t.Fatal("expected model change to change embedding fingerprint")
	}
	if first == embeddingFingerprintForConfig(base, 1024) {
		t.Fatal("expected vector dimension change to change embedding fingerprint")
	}
}

func TestDocumentEmbeddingFingerprintMismatchRequiresReindex(t *testing.T) {
	config := model.EmbeddingConfig{Provider: "ollama", BaseURL: "http://localhost:11434", Model: "embed-a"}
	current := embeddingFingerprintForConfig(config, 768)
	if !documentEmbeddingFingerprintNeedsReindex(model.Document{Status: "indexed"}, current) {
		t.Fatal("expected a legacy indexed document without fingerprint to require reindex")
	}
	if documentEmbeddingFingerprintNeedsReindex(model.Document{Status: "processing", EmbeddingFingerprint: current}, current) {
		t.Fatal("processing document should not be marked for embedding fingerprint reindex")
	}
	if documentEmbeddingFingerprintNeedsReindex(model.Document{Status: "indexed", EmbeddingFingerprint: current}, current) {
		t.Fatal("matching embedding fingerprint should not require reindex")
	}
}
