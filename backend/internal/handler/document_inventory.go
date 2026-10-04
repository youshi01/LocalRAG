package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"localrag/internal/model"
	"localrag/internal/service"
	"net/http"
	"time"
)

// serveDocumentInventory returns true only when this metadata query was handled.
// It never invokes chat, embeddings, vector recall, or body evidence matching.
func (h *AppHandler) serveDocumentInventory(c *gin.Context, req model.ChatCompletionRequest, stream bool) bool {
	result, matched, err := h.appService.BuildDocumentInventoryAnswer(req)
	if !matched {
		return false
	}
	if err != nil {
		writeChatPreparationError(c, err)
		return true
	}
	metadata := documentInventoryMetadata(req, result)
	// Persist before emitting done; a database failure must not be announced as
	// a successful completed stream.
	if _, err = h.appService.SaveConversation(model.SaveConversationRequest{ID: req.ConversationID, KnowledgeBaseID: req.KnowledgeBaseID, DocumentID: req.DocumentID, Messages: buildStoredConversationMessages(req.Messages, result.Content, metadata)}); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return true
	}
	if !stream {
		c.JSON(http.StatusOK, model.ChatCompletionResponse{ID: fmt.Sprintf("inventory-%d", time.Now().UnixNano()), Object: "chat.completion", Created: time.Now().Unix(), Model: "localrag-document-inventory", Choices: []model.ChatCompletionChoice{{Index: 0, Message: model.ChatMessage{Role: "assistant", Content: result.Content}}}, Metadata: metadata})
		return true
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, "streaming is not supported")
		return true
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.SSEvent("meta", metadata)
	c.SSEvent("chunk", gin.H{"content": result.Content})
	c.SSEvent("done", gin.H{"content": result.Content, "metadata": metadata})
	flusher.Flush()
	return true
}

func documentInventoryMetadata(req model.ChatCompletionRequest, result service.DocumentInventoryAnswer) map[string]any {
	status, coverage := "supported", 1.0
	if result.NeedsScope {
		status, coverage = "abstained", 0
	}
	return map[string]any{
		"sources": result.Sources, "knowledgeBaseId": req.KnowledgeBaseID, "documentId": req.DocumentID,
		"toolUse":           buildToolUseMetadata(result.Sources),
		"citationSupport":   map[string]any{"status": status, "basis": "document_inventory", "summary": "文件清单来自当前范围已保存的目录，按目录记录逐项核对；不代表已验证文档正文。", "claimCount": result.Count, "supportedClaimCount": result.Count, "coverage": coverage},
		"documentInventory": map[string]any{"count": result.Count, "complete": !result.NeedsScope},
	}
}
