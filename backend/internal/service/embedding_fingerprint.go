package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"localrag/internal/model"
)

func embeddingFingerprintForConfig(config model.EmbeddingConfig, vectorSize int) string {
	provider, baseURL, err := normalizeModelEndpoint(config.Provider, config.BaseURL)
	if err != nil {
		provider = strings.ToLower(strings.TrimSpace(config.Provider))
		baseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	}
	canonical := fmt.Sprintf("%s\x00%s\x00%s\x00%d", provider, baseURL, strings.TrimSpace(config.Model), vectorSize)
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

func documentEmbeddingFingerprintNeedsReindex(document model.Document, currentFingerprint string) bool {
	if strings.TrimSpace(document.Status) != "indexed" {
		return false
	}
	if strings.TrimSpace(currentFingerprint) == "" {
		return false
	}
	return strings.TrimSpace(document.EmbeddingFingerprint) == "" ||
		strings.TrimSpace(document.EmbeddingFingerprint) != strings.TrimSpace(currentFingerprint)
}

func (s *AppService) currentEmbeddingFingerprint() string {
	if s == nil || s.state == nil {
		return ""
	}
	s.state.Mu.RLock()
	config := s.state.Config.Embedding
	s.state.Mu.RUnlock()
	if strings.TrimSpace(config.Provider) == "" || strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.Model) == "" {
		return ""
	}
	return embeddingFingerprintForConfig(config, s.qdrantVectorSize())
}
