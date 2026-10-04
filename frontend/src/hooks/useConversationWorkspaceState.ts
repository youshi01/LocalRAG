import { useCallback, useState } from 'react'
import { ConversationOperationGuard, clearConversationWorkspace } from '../app/conversationReset'
import type { CitationNavigationTarget, Conversation, WorkspaceView } from '../App'

export const useConversationWorkspaceState = (
  createInitialConversation: () => Conversation,
) => {
  const [conversations, setConversations] = useState<Conversation[]>(() => [
    createInitialConversation(),
  ])
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null)
  const [activeWorkspace, setActiveWorkspace] = useState<WorkspaceView>('chat')
  const [citationNavigationTarget, setCitationNavigationTarget] =
    useState<CitationNavigationTarget | null>(null)
  const [streamingConversationId, setStreamingConversationId] = useState<string | null>(null)
  const [conversationOperationGuard] = useState(() => new ConversationOperationGuard())
  const [conversationResetKey, setConversationResetKey] = useState(0)
  const [isClearingConversations, setIsClearingConversations] = useState(false)
  const [isConversationMutating, setIsConversationMutating] = useState(false)

  const runConversationMutation = useCallback(async <T,>(
    operation: () => Promise<T>,
    blockedResult: T,
  ): Promise<T> => {
    // Old rendered callbacks must not save an already-deleted conversation.
    if (!conversationOperationGuard.isCurrent(conversationResetKey)) return blockedResult
    const release = conversationOperationGuard.beginMutation()
    if (!release) return blockedResult
    setIsConversationMutating(true)
    try {
      return await operation()
    } finally {
      release()
      setIsConversationMutating(conversationOperationGuard.isMutating)
    }
  }, [conversationOperationGuard, conversationResetKey])

  const clearConversations = useCallback(async (knowledgeBaseId: string, documentId: string) => {
    if (!conversationOperationGuard.isCurrent(conversationResetKey)) return null
    const pending = clearConversationWorkspace(
      conversationOperationGuard,
      { knowledgeBaseId, documentId },
      (snapshot) => {
        // Conversations are the list, loaded-detail and message-history cache.
        setConversations(snapshot.conversations)
        setActiveConversationId(snapshot.activeConversationId)
        setStreamingConversationId(snapshot.streamingConversationId)
        setCitationNavigationTarget(snapshot.citationNavigationTarget)
        setConversationResetKey(snapshot.resetKey)
      },
    )
    setIsClearingConversations(conversationOperationGuard.isClearing)
    try {
      return await pending
    } finally {
      setIsClearingConversations(conversationOperationGuard.isClearing)
    }
  }, [conversationOperationGuard, conversationResetKey])

  return {
    conversations,
    setConversations,
    activeConversationId,
    setActiveConversationId,
    activeWorkspace,
    setActiveWorkspace,
    citationNavigationTarget,
    setCitationNavigationTarget,
    streamingConversationId,
    setStreamingConversationId,
    conversationOperationGuard,
    conversationResetKey,
    isClearingConversations,
    isConversationMutating,
    runConversationMutation,
    clearConversations,
  }
}
