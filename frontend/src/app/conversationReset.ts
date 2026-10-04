import type { CitationNavigationTarget, Conversation } from '../App'
import { clearAllConversations } from '../services/api'
import { createEmptyConversation } from './appHelpers'

interface ClearAllEligibilityFlags {
  isGenerating?: boolean
  isClearing?: boolean
  isMutating?: boolean
}

export const getClearAllConversationState = (
  conversations: Conversation[],
  { isGenerating = false, isClearing = false, isMutating = false }: ClearAllEligibilityFlags = {},
) => {
  // Remote summaries have no loaded messages yet. Only untouched, empty local
  // placeholders are excluded; named drafts and local messages still matter.
  const meaningfulCount = conversations.filter((conversation) => (
    !conversation.localOnly ||
    conversation.messages.length > 0 ||
    (conversation.title.trim() !== '' && conversation.title.trim() !== '新的对话')
  )).length
  const disabledReason = isClearing
    ? '正在清空本项目全部会话，请稍候。'
    : isGenerating
      ? '当前正在生成，请等待完成后再清空。'
      : isMutating
        ? '当前有会话操作正在进行，请等待完成后再清空。'
        : meaningfulCount === 0
          ? '暂无可清空的会话。'
          : ''

  return { totalCount: conversations.length, meaningfulCount, canClear: !disabledReason, disabledReason }
}

// Shared by bootstrap/detail loaders and mutations. A successful clear advances
// the epoch; late reads cannot restore old state, and saves cannot cross it.
export class ConversationOperationGuard {
  private currentRevision = 0
  private clearing = false
  private pendingMutations = 0

  get revision() { return this.currentRevision }
  get isClearing() { return this.clearing }
  get isMutating() { return this.pendingMutations > 0 }

  isCurrent(revision: number) {
    return revision === this.currentRevision && !this.clearing
  }

  beginMutation(): (() => void) | null {
    if (this.clearing) return null
    this.pendingMutations += 1
    let released = false
    return () => {
      if (released) return
      released = true
      this.pendingMutations -= 1
    }
  }

  beginClear() {
    if (this.clearing || this.isMutating) return false
    this.clearing = true
    return true
  }

  finishClear(success: boolean) {
    if (!this.clearing) return
    if (success) this.currentRevision += 1
    this.clearing = false
  }
}

export const loadCurrentConversationData = async <T>(
  guard: ConversationOperationGuard,
  load: () => Promise<T>,
): Promise<T | null> => {
  const revision = guard.revision
  if (!guard.isCurrent(revision)) return null
  try {
    const data = await load()
    return guard.isCurrent(revision) ? data : null
  } catch (error) {
    if (!guard.isCurrent(revision)) return null
    throw error
  }
}

export interface ClearedConversationWorkspace {
  conversations: Conversation[]
  activeConversationId: string
  streamingConversationId: null
  citationNavigationTarget: CitationNavigationTarget | null
  resetKey: number
}

export const clearConversationWorkspace = async (
  guard: ConversationOperationGuard,
  scope: { knowledgeBaseId: string; documentId: string },
  onCleared: (snapshot: ClearedConversationWorkspace) => void,
) => {
  if (!guard.beginClear()) return null
  try {
    const result = await clearAllConversations()
    const conversation = createEmptyConversation(scope.knowledgeBaseId, scope.documentId)
    guard.finishClear(true)
    onCleared({
      conversations: [conversation],
      activeConversationId: conversation.id,
      streamingConversationId: null,
      citationNavigationTarget: null,
      resetKey: guard.revision,
    })
    return result
  } finally {
    guard.finishClear(false)
  }
}
