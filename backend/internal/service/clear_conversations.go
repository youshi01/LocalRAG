package service

import (
	"errors"
	"fmt"
)

var ErrConversationGenerationActive = errors.New("有会话正在生成或保存，请等待结束后再清空全部会话")

type bulkConversationDeleter interface{ DeleteAllConversations() (int, error) }

// BeginConversationRequest protects a generation/mutation through its final
// save. Bulk clearing never waits for a long-running model: it rejects instead.
func (s *AppService) BeginConversationRequest() func() {
	if s == nil {
		return func() {}
	}
	s.conversationLifecycleMu.RLock()
	return s.conversationLifecycleMu.RUnlock
}

func (s *AppService) DeleteAllConversations() (int, error) {
	if s == nil {
		return 0, fmt.Errorf("app service is nil")
	}
	if !s.conversationLifecycleMu.TryLock() {
		return 0, ErrConversationGenerationActive
	}
	defer s.conversationLifecycleMu.Unlock()
	store, ok := s.chatHistory.(bulkConversationDeleter)
	if !ok {
		return 0, fmt.Errorf("chat history store does not support atomic bulk deletion")
	}
	return store.DeleteAllConversations()
}
