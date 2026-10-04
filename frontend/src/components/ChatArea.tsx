import React, { useEffect, useMemo, useRef, useState } from 'react'
import type {
  AppConfig,
  ChatMode,
  ChatContentMode,
  Conversation,
  DocumentItem,
  ChatSourceMetadata,
  KnowledgeBase,
} from '../App'
import MessageCard from './chat/MessageCard'
import ConversationExportDialog from './chat/ConversationExportDialog'
import ConfirmDialog from './common/ConfirmDialog'
import AppIcon, { type AppIconName } from './common/AppIcon'

interface ChatAreaProps {
  sidebarOpen: boolean
  activeConversation: Conversation
  selectedKnowledgeBase: KnowledgeBase | null
  selectedDocument: DocumentItem | null
  config: AppConfig
  chatMode: ChatMode
  isLoading: boolean
  isGlobalGenerating: boolean
  generatingConversationTitle: string
  enforceSingleFlight: boolean
  onChatModeChange: (mode: ChatMode) => void
  contentMode: ChatContentMode
  supportsFullTableMode: boolean
  onContentModeChange: (mode: ChatContentMode) => void
  onSendMessage: (content: string) => Promise<boolean>
  onClearConversation: () => void
  onEditMessage?: (messageId: string, newContent: string) => Promise<void>
  onDeleteMessage?: (messageId: string) => Promise<void>
  onRegenerateMessage?: (messageId: string) => Promise<void>
  onExportConversation?: (conversationId: string, format: 'markdown') => Promise<string>
  onOpenCitationSource?: (source: ChatSourceMetadata) => void
}

const suggestedPrompts = [
  '请总结当前知识库的核心观点',
  '请列出这个知识库中最关键的结论',
  '如果基于当前资料开始实现，下一步建议是什么？',
]

type ChatIconName = 'bolt' | 'brain' | 'clock' | 'database' | 'file' | 'message' | 'send' | 'user'

const ChatIcon: React.FC<{ name: ChatIconName }> = ({ name }) => {
  const iconMap: Record<ChatIconName, AppIconName> = {
    bolt: 'zap',
    brain: 'brain',
    clock: 'clock',
    database: 'database',
    file: 'file',
    message: 'message',
    send: 'send',
    user: 'user',
  }
  return <AppIcon name={iconMap[name]} />
}

const ChatArea: React.FC<ChatAreaProps> = ({
  sidebarOpen,
  activeConversation,
  selectedKnowledgeBase,
  selectedDocument,
  config,
  chatMode,
  isLoading,
  isGlobalGenerating,
  generatingConversationTitle,
  enforceSingleFlight,
  onChatModeChange,
  contentMode,
  supportsFullTableMode,
  onContentModeChange,
  onSendMessage,
  onClearConversation,
  onEditMessage,
  onDeleteMessage,
  onRegenerateMessage,
  onExportConversation,
  onOpenCitationSource,
}) => {
  const [inputValue, setInputValue] = useState('')
  const [copiedMessageId, setCopiedMessageId] = useState<string | null>(null)
  const [showClearConfirm, setShowClearConfirm] = useState(false)
  const [showExportDialog, setShowExportDialog] = useState(false)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState(activeConversation.title)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)

  const canSend = inputValue.trim().length > 0 && !(enforceSingleFlight && isGlobalGenerating)

  const hasMessages = activeConversation.messages.length > 0

  // Auto-resize textarea
  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = 'auto'
    const lineHeight = 22
    const maxHeight = lineHeight * 6
    textarea.style.height = `${Math.min(textarea.scrollHeight, maxHeight)}px`
  }, [inputValue])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [activeConversation.messages, isLoading])

  const conversationStats = useMemo(() => {
    const userCount = activeConversation.messages.filter(
      (message) => message.role === 'user',
    ).length

    return {
      userCount,
      totalCount: activeConversation.messages.length,
    }
  }, [activeConversation.messages])

  const hasUserMessages = conversationStats.userCount > 0
  const canUseMessageActions = !isGlobalGenerating
  const canExportConversation = Boolean(onExportConversation) && hasUserMessages && !isGlobalGenerating

  const scopeText = selectedDocument
    ? `文档问答：${selectedDocument.name}`
    : selectedKnowledgeBase
      ? `知识库问答：${selectedKnowledgeBase.name}`
      : '未选择知识库'
  const knowledgeBaseBadgeText = selectedKnowledgeBase?.name ?? '未选择知识库'
  const retrievalScopeText = selectedDocument
    ? `文档：${selectedDocument.name}`
    : selectedKnowledgeBase
      ? '全部文档'
      : '未选择范围'
  const retrievalScopeTitle = selectedDocument
    ? `当前检索范围：单独文档「${selectedDocument.name}」`
    : selectedKnowledgeBase
      ? `当前检索范围：知识库「${selectedKnowledgeBase.name}」的全部文档`
      : '当前检索范围：未选择'

  const activeChatModel = config.chat.model

  const handleSubmit = async () => {
    const content = inputValue.trim()
    if (!content || isLoading) {
      return
    }

    setInputValue('')
    const accepted = await onSendMessage(content)
    if (!accepted) {
      setInputValue((current) => current || content)
    }
  }

  const handleKeyDown = async (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      await handleSubmit()
    }
  }

  const handleCopyMessage = async (messageId: string, content: string) => {
    try {
      await navigator.clipboard.writeText(content)
      setCopiedMessageId(messageId)
      window.setTimeout(() => {
        setCopiedMessageId((prev) => (prev === messageId ? null : prev))
      }, 1500)
    } catch {
      // 忽略复制异常，避免影响主流程
    }
  }

  const handleConfirmClearConversation = () => {
    setShowClearConfirm(false)
    onClearConversation()
  }

  return (
    <main className={`chat-area ${sidebarOpen ? 'sidebar-open' : 'sidebar-closed'}`}>
      <div className="chat-topbar">
        <div className="chat-topbar-main">
          <div className="chat-topbar-left">
            {editingTitle ? (
              <input
                className="chat-topbar-title-input"
                value={titleDraft}
                onChange={(e) => setTitleDraft(e.target.value)}
                onBlur={() => setEditingTitle(false)}
                onKeyDown={(e) => { if (e.key === 'Enter') setEditingTitle(false) }}
                autoFocus
              />
            ) : (
              <span
                className="chat-topbar-title"
                onDoubleClick={() => { setTitleDraft(activeConversation.title); setEditingTitle(true) }}
                title="双击编辑标题"
              >
                {activeConversation.title}
              </span>
            )}
          </div>

          <div className="chat-context-summary" title={retrievalScopeTitle}>
            <ChatIcon name="database" />
            <span>{knowledgeBaseBadgeText}</span>
            <span className="chat-context-separator">/</span>
            <span>{retrievalScopeText}</span>
          </div>

          <div className="chat-topbar-right">
            {enforceSingleFlight && isGlobalGenerating && (
              <span className="chat-topbar-hint" aria-live="polite">
                生成中：{generatingConversationTitle}
              </span>
            )}
            <span
              className="chat-model-status"
              title={`${chatMode === 'think' ? '思考模式' : '普通回答'} · ${activeChatModel}`}
            >
              <ChatIcon name={chatMode === 'think' ? 'brain' : 'bolt'} />
              <span>{activeChatModel}</span>
            </span>
            {supportsFullTableMode && (
              <select
                aria-label="回答模式"
                className="chat-content-mode"
                value={contentMode}
                onChange={(event) => onContentModeChange(event.target.value as ChatContentMode)}
                title="完整表格模式会直接读取 CSV/XLSX 的表头和数据行"
              >
                <option value="default">普通问答</option>
                <option value="full_table">完整表格查询</option>
              </select>
            )}
            <div className="chat-topbar-actions" aria-label="对话操作">
              <button
                type="button"
                className="chat-topbar-action-btn chat-topbar-export-btn"
                onClick={() => setShowExportDialog(true)}
                disabled={!canExportConversation}
                title={canExportConversation ? '导出对话' : '暂无可导出的对话'}
                aria-label="导出对话"
              >
                <AppIcon name="download" size={17} />
              </button>
              <button
                type="button"
                className="chat-topbar-action-btn chat-topbar-clear-btn"
                onClick={() => setShowClearConfirm(true)}
                disabled={isLoading}
                title="清空对话"
                aria-label="清空对话"
              >
                <AppIcon name="trash" size={17} />
              </button>
            </div>
          </div>
        </div>
      </div>

      <div className="messages-container">
        {activeConversation.messages.length === 0 ? (
          <div className="welcome-message">
            <span className="welcome-mark"><AppIcon name="sparkles" size={22} /></span>
            <h2>开始本地对话</h2>
            <p>选择知识范围后直接提问，回答与引用会保留在当前会话中。</p>
          </div>
        ) : (
          activeConversation.messages.map((message, index) => {
            const isStreamingPlaceholder =
              isLoading &&
              message.role === 'assistant' &&
              message.id === activeConversation.messages.at(-1)?.id &&
              !message.content.trim()
            const previousMessage = activeConversation.messages[index - 1]
            const canDeleteMessage =
              canUseMessageActions && activeConversation.messages.length > 1
            const canRegenerateMessage =
              canUseMessageActions &&
              message.role === 'assistant' &&
              previousMessage?.role === 'user'

            return (
              <MessageCard
                key={message.id}
                message={message}
                isLoading={isLoading}
                isStreamingPlaceholder={isStreamingPlaceholder}
                onCopyMessage={handleCopyMessage}
                onEditMessage={canUseMessageActions ? onEditMessage : undefined}
                onDeleteMessage={canDeleteMessage ? onDeleteMessage : undefined}
                onRegenerateMessage={
                  canRegenerateMessage ? onRegenerateMessage : undefined
                }
                onOpenCitationSource={onOpenCitationSource}
                copiedMessageId={copiedMessageId}
              />
            )
          })
        )}

        {isLoading && activeConversation.messages.at(-1)?.role !== 'assistant' && (
          <div className="message assistant loading">
            <div className="message-content">AI 正在生成回答...</div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {!hasMessages && (
        <div className="prompt-list">
          {suggestedPrompts.map((prompt) => (
            <button
              key={prompt}
              type="button"
              className="prompt-chip"
              disabled={enforceSingleFlight && isGlobalGenerating}
              onClick={() => {
                void onSendMessage(prompt).then((accepted) => {
                  if (!accepted) {
                    setInputValue((current) => current || prompt)
                  }
                })
              }}
            >
              {prompt}
            </button>
          ))}
        </div>
      )}

      <div className="input-area">
        <div className="input-context-row">
          <span className="input-context-icon">
            <ChatIcon name="database" />
          </span>
          <span className="input-context-text">{scopeText}</span>
          {selectedDocument && (
            <span className="input-context-badge">文档</span>
          )}
          <button
            type="button"
            className="chat-thinking-switch"
            role="switch"
            aria-label="思考模式"
            aria-checked={chatMode === 'think'}
            disabled={isGlobalGenerating}
            onClick={() => onChatModeChange(chatMode === 'think' ? 'fast' : 'think')}
            title={'思考模式' + (chatMode === 'think' ? '已开启' : '已关闭') + ' · 共用聊天模型 ' + activeChatModel + '；需要模型服务支持思考控制'}
          >
            <ChatIcon name="brain" />
            <span>思考</span>
            <span className="chat-thinking-switch-track" aria-hidden="true" />
          </button>
        </div>
        <div className="input-container">
          <textarea
            ref={textareaRef}
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={
              enforceSingleFlight && isGlobalGenerating
                ? `当前正在后台生成「${generatingConversationTitle}」，请等待完成后再发送`
                : '输入您的问题，Enter 发送，Shift + Enter 换行'
            }
            rows={1}
          />
          <button
            type="button"
            onClick={() => {
              void handleSubmit()
            }}
            disabled={!canSend}
            className={`send-btn ${canSend ? 'send-btn-active' : ''}`}
            aria-label="发送消息"
          >
            {isLoading ? <span className="send-loading-dot" /> : <AppIcon name="send" size={18} />}
          </button>
        </div>
      </div>

      <ConfirmDialog
        open={showClearConfirm}
        title="清空对话"
        message="确认清空当前对话的全部消息吗？此操作不可撤销。"
        confirmText="清空"
        cancelText="取消"
        onConfirm={handleConfirmClearConversation}
        onCancel={() => setShowClearConfirm(false)}
      />

      {onExportConversation && (
        <ConversationExportDialog
          conversation={activeConversation}
          isOpen={showExportDialog}
          onClose={() => setShowExportDialog(false)}
          onExport={onExportConversation}
        />
      )}
    </main>
  )
}

export default ChatArea
