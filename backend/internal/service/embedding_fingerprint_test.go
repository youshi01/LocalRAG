package service

import (
	"context"
	"testing"
	"time"

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

func TestCommitIndexOperationPersistsEmbeddingFingerprintWithoutLockingState(t *testing.T) {
	service := &AppService{state: &model.AppState{
		Config: model.AppConfig{Embedding: model.EmbeddingConfig{
			Provider: "ollama",
			BaseURL:  "http://127.0.0.1:11434",
			Model:    "embed-test",
		}},
		KnowledgeBases: map[string]model.KnowledgeBase{
			"kb-1": {ID: "kb-1"},
		},
	}}

	op, err := service.beginIndexOperation(context.Background(), model.Document{
		ID:              "doc-1",
		KnowledgeBaseID: "kb-1",
		Name:            "notes.md",
		Status:          "processing",
		Version:         1,
	})
	if err != nil {
		t.Fatalf("begin index operation: %v", err)
	}

	type commitResult struct {
		document model.Document
		err      error
	}
	result := make(chan commitResult, 1)
	go func() {
		document, err := service.commitIndexOperation(context.Background(), op, model.Document{
			ID:              op.DocumentID,
			KnowledgeBaseID: op.KnowledgeBaseID,
			Name:            "notes.md",
			Status:          "indexed",
			Version:         1,
			IndexFence:      op.Fence,
		}, "upload", time.Now().UTC())
		result <- commitResult{document: document, err: err}
	}()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("commit index operation: %v", got.err)
		}
		expected := embeddingFingerprintForConfig(service.state.Config.Embedding, service.qdrantVectorSize())
		if got.document.EmbeddingFingerprint != expected {
			t.Fatalf("expected committed document fingerprint %q, got %q", expected, got.document.EmbeddingFingerprint)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("commit index operation timed out while reading embedding configuration")
	}
}
