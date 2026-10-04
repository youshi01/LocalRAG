package service

import (
	"fmt"
	"localrag/internal/model"
	"strings"
	"testing"
)

func TestDocumentInventoryCompleteBeyondPreviewAndTopK(t *testing.T) {
	docs := make([]model.Document, 12)
	for i := range docs {
		docs[i] = model.Document{ID: fmt.Sprintf("doc-%d", i), Name: fmt.Sprintf("file-%02d.txt", i), Status: "ready"}
	}
	svc := &AppService{state: &model.AppState{KnowledgeBases: map[string]model.KnowledgeBase{"kb": {ID: "kb", Name: "完整目录", Documents: docs}}}}
	result, matched, err := svc.BuildDocumentInventoryAnswer(model.ChatCompletionRequest{KnowledgeBaseID: "kb", Messages: []model.ChatMessage{{Role: "user", Content: "列出全部文件"}}})
	if err != nil || !matched || result.Count != 12 || len(result.Sources) != 12 {
		t.Fatalf("directory incomplete: %+v %v %v", result, matched, err)
	}
	for _, d := range docs {
		if !strings.Contains(result.Content, d.Name) {
			t.Errorf("missing %s", d.Name)
		}
	}
}

func TestDocumentInventoryNoSelectedScopeDoesNotEnumerateEverything(t *testing.T) {
	svc := &AppService{state: &model.AppState{KnowledgeBases: map[string]model.KnowledgeBase{"secret": {ID: "secret", Name: "不应展开", Documents: []model.Document{{ID: "doc", Name: "private.txt"}}}}}}
	result, matched, err := svc.BuildDocumentInventoryAnswer(model.ChatCompletionRequest{Messages: []model.ChatMessage{{Role: "user", Content: "列出知识库文件"}}})
	if err != nil || !matched || !result.NeedsScope || strings.Contains(result.Content, "private.txt") {
		t.Fatalf("unexpected scope expansion: %+v %v", result, err)
	}
}

func TestInventoryFileNamesAreMarkdownData(t *testing.T) {
	name := "[link](https://example.invalid)" + string(rune(96)) + "note.txt"
	output := inventoryFileNameMarkdown(name)
	if !strings.Contains(output, name) || !strings.HasPrefix(output, strings.Repeat(string(rune(96)), 2)+" ") {
		t.Fatalf("unsafe or lossy name formatting: %q", output)
	}
}
