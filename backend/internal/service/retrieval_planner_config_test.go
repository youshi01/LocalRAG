package service

import (
	"strings"
	"testing"

	"localrag/internal/model"
)

func TestNormalizeRetrievalConfigDefaultsModelPlannerForLegacyState(t *testing.T) {
	serverConfig := model.ServerConfig{
		EnableModelRetrievalPlanner: true,
		RetrievalPlannerMaxRounds:   2,
	}
	legacy := model.RetrievalConfig{DefaultSearchMode: "dense", TopKDocument: 8}
	normalized := normalizeRetrievalConfig(legacy, serverConfig)
	if !normalized.EnableModelRetrievalPlanner || normalized.ModelRetrievalMaxRounds != 2 {
		t.Fatalf("expected legacy config to inherit planner defaults, got %#v", normalized)
	}
}

func TestNormalizeRetrievalConfigPreservesExplicitPlannerDisable(t *testing.T) {
	serverConfig := model.ServerConfig{EnableModelRetrievalPlanner: true, RetrievalPlannerMaxRounds: 2}
	disabled := model.RetrievalConfig{
		DefaultSearchMode:           "dense",
		EnableModelRetrievalPlanner: false,
		ModelRetrievalMaxRounds:     2,
		TopKDocument:                8,
	}
	normalized := normalizeRetrievalConfig(disabled, serverConfig)
	if normalized.EnableModelRetrievalPlanner {
		t.Fatalf("expected explicit planner disable to be preserved, got %#v", normalized)
	}
	if normalized.ModelRetrievalMaxRounds != 2 {
		t.Fatalf("expected two planner rounds, got %d", normalized.ModelRetrievalMaxRounds)
	}
}

func TestMergeRetrievedChunksForPlannerKeepsHighestScore(t *testing.T) {
	chunks := mergeRetrievedChunksForPlanner([]RetrievedChunk{
		{DocumentChunk: DocumentChunk{ID: "a"}, Score: 0.2},
		{DocumentChunk: DocumentChunk{ID: "b"}, Score: 0.4},
		{DocumentChunk: DocumentChunk{ID: "a"}, Score: 0.9},
	})
	if len(chunks) != 2 {
		t.Fatalf("expected two unique chunks, got %#v", chunks)
	}
	if chunks[0].ID != "a" || chunks[0].Score != 0.9 {
		t.Fatalf("expected highest score for a first, got %#v", chunks)
	}
}

func TestRetrievalEvidenceSummaryIsBoundedAndClearlyData(t *testing.T) {
	chunks := []RetrievedChunk{{DocumentChunk: DocumentChunk{
		DocumentName: "文档.md",
		Text:         strings.Repeat("资料片段。", 1000),
	}, Score: 0.8}}
	summary := retrievalEvidenceSummary(chunks)
	if len([]rune(summary)) > 600 {
		t.Fatalf("expected evidence summary to stay bounded, got %d runes", len([]rune(summary)))
	}
	if !strings.Contains(summary, "文档.md") || !strings.Contains(summary, "score=0.8000") {
		t.Fatalf("expected bounded evidence metadata, got %q", summary)
	}
}
