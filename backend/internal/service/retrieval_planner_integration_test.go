package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"localrag/internal/model"
)

type sequenceRetrievalPlanner struct {
	mu     sync.Mutex
	plans  []RetrievalPlan
	errors []error
	inputs []RetrievalPlanInput
}

func (p *sequenceRetrievalPlanner) Plan(_ context.Context, input RetrievalPlanInput) (RetrievalPlan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inputs = append(p.inputs, input)
	index := len(p.inputs) - 1
	if index < len(p.errors) && p.errors[index] != nil {
		return RetrievalPlan{}, p.errors[index]
	}
	if index >= len(p.plans) {
		return RetrievalPlan{NeedKnowledge: true, Queries: []string{"补充资料"}}, nil
	}
	return p.plans[index], nil
}

func (p *sequenceRetrievalPlanner) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.inputs)
}

func newPlannerRetrievalTestService(t *testing.T, planner RetrievalPlanner, embeddingServer *httptest.Server, qdrantServer *httptest.Server, rounds int) *AppService {
	t.Helper()
	state := &model.AppState{
		Config: model.AppConfig{
			Chat:      model.ChatConfig{Provider: "openai-compatible", BaseURL: "http://planner.invalid/v1", Model: "chat-model"},
			Embedding: model.EmbeddingConfig{Provider: "openai-compatible", BaseURL: embeddingServer.URL + "/v1", Model: "embed-model"},
			Retrieval: model.RetrievalConfig{
				DefaultSearchMode:           "dense",
				RerankStrategy:              "keyword",
				EnableModelRetrievalPlanner: true,
				ModelRetrievalMaxRounds:     rounds,
				TopKKnowledgeBase:           4,
				CandidateTopKAllDocs:        8,
				MaxChunksPerDocument:        3,
				MaxContextChars:             4000,
			},
		},
		KnowledgeBases: map[string]model.KnowledgeBase{
			"kb-1": {
				ID:        "kb-1",
				Documents: []model.Document{{ID: "doc-1", KnowledgeBaseID: "kb-1", Name: "资料.md"}},
			},
		},
	}
	service := &AppService{
		state: state,
		serverConfig: model.ServerConfig{
			QdrantURL:                   qdrantServer.URL,
			QdrantCollectionPrefix:      "kb_",
			QdrantVectorSize:            2,
			EnableModelRetrievalPlanner: true,
			RetrievalPlannerMaxRounds:   rounds,
		},
		qdrant: NewQdrantService(model.ServerConfig{
			QdrantURL:              qdrantServer.URL,
			QdrantCollectionPrefix: "kb_",
			QdrantVectorSize:       2,
		}),
		rag: NewRagService(),
	}
	service.rag.SetQdrantService(service.qdrant)
	service.SetRetrievalPlanner(planner)
	return service
}

func newEmptyRetrievalBackend(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		var request struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data := make([]map[string]any, 0, len(request.Input))
		for index := range request.Input {
			data = append(data, map[string]any{"index": index, "embedding": []float64{1, 0}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/points/query") && !strings.Contains(r.URL.Path, "/points/search") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}})
	}))
	return embeddingServer, qdrantServer
}

func TestEvaluatePlannerCanSkipKnowledgeRetrieval(t *testing.T) {
	embeddingServer, qdrantServer := newEmptyRetrievalBackend(t)
	t.Cleanup(embeddingServer.Close)
	t.Cleanup(qdrantServer.Close)
	planner := &sequenceRetrievalPlanner{plans: []RetrievalPlan{{NeedKnowledge: false}}}
	service := newPlannerRetrievalTestService(t, planner, embeddingServer, qdrantServer, 2)

	chunks, err := service.EvaluateRetrieveWithContext(t.Context(), model.ChatCompletionRequest{
		KnowledgeBaseID: "kb-1",
		Messages:        []model.ChatMessage{{Role: "user", Content: "你好，介绍一下你自己"}},
	})
	if err != nil {
		t.Fatalf("evaluate retrieval: %v", err)
	}
	if len(chunks) != 0 || planner.callCount() != 1 {
		t.Fatalf("expected planner-only skip, chunks=%#v calls=%d", chunks, planner.callCount())
	}
}

func TestEvaluateUsesOriginalAndPlannerQueries(t *testing.T) {
	embeddingServer, qdrantServer := newEmptyRetrievalBackend(t)
	t.Cleanup(embeddingServer.Close)
	t.Cleanup(qdrantServer.Close)
	planner := &sequenceRetrievalPlanner{plans: []RetrievalPlan{{NeedKnowledge: true, Queries: []string{"已有建设内容"}}}}
	service := newPlannerRetrievalTestService(t, planner, embeddingServer, qdrantServer, 1)

	chunks, err := service.EvaluateRetrieveWithContext(t.Context(), model.ChatCompletionRequest{
		KnowledgeBaseID: "kb-1",
		Messages:        []model.ChatMessage{{Role: "user", Content: "请根据当前资料总结下一阶段实施工作"}},
	})
	if err != nil {
		t.Fatalf("evaluate retrieval: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected empty mock result, got %#v", chunks)
	}
	if planner.callCount() != 1 {
		t.Fatalf("expected one planner call, got %d", planner.callCount())
	}
}

func TestEvaluatePlannerFailureFallsBackToExistingRetrieval(t *testing.T) {
	embeddingServer, qdrantServer := newEmptyRetrievalBackend(t)
	t.Cleanup(embeddingServer.Close)
	t.Cleanup(qdrantServer.Close)
	planner := &sequenceRetrievalPlanner{errors: []error{errors.New("planner unavailable")}}
	service := newPlannerRetrievalTestService(t, planner, embeddingServer, qdrantServer, 2)

	chunks, err := service.EvaluateRetrieveWithContext(t.Context(), model.ChatCompletionRequest{
		KnowledgeBaseID: "kb-1",
		Messages:        []model.ChatMessage{{Role: "user", Content: "请根据当前资料总结下一阶段实施工作"}},
	})
	if err != nil {
		t.Fatalf("expected deterministic fallback instead of planner error: %v", err)
	}
	if len(chunks) != 0 || planner.callCount() != 1 {
		t.Fatalf("unexpected fallback result: chunks=%#v calls=%d", chunks, planner.callCount())
	}
}

func TestEvaluatePlannerPerformsAtMostOneFollowUpRound(t *testing.T) {
	embeddingServer, qdrantServer := newEmptyRetrievalBackend(t)
	t.Cleanup(embeddingServer.Close)
	t.Cleanup(qdrantServer.Close)
	planner := &sequenceRetrievalPlanner{plans: []RetrievalPlan{
		{NeedKnowledge: true, Queries: []string{"首轮主题"}},
		{NeedKnowledge: true, Queries: []string{"补充主题"}},
		{NeedKnowledge: true, Queries: []string{"不应执行的第三轮"}},
	}}
	service := newPlannerRetrievalTestService(t, planner, embeddingServer, qdrantServer, 2)

	_, err := service.EvaluateRetrieveWithContext(t.Context(), model.ChatCompletionRequest{
		KnowledgeBaseID: "kb-1",
		Messages:        []model.ChatMessage{{Role: "user", Content: "请总结当前资料的核心观点"}},
	})
	if err != nil {
		t.Fatalf("evaluate retrieval: %v", err)
	}
	if calls := planner.callCount(); calls != 2 {
		t.Fatalf("expected exactly two planner calls, got %d", calls)
	}
}
