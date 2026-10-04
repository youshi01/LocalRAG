import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { createDefaultAppConfig } from '../../app/appConfig'
import ChatArea from '../ChatArea'
import AISettings from '../settings/tabs/AISettings'
import GeneralSettings from '../settings/tabs/GeneralSettings'

const config = createDefaultAppConfig()
config.chat.model = 'configured-chat-model'
const legacySelection = {
  chatModeSettings: { fastModel: 'obsolete-fast-model', thinkModel: 'obsolete-think-model' },
  onThinkModelChange: () => undefined,
}

describe('single-model thinking switch', () => {
  it('removes the separate thinking model input from model settings', () => {
    const html = renderToStaticMarkup(createElement(AISettings, {
      config, onChatConfigChange: () => undefined, onEmbeddingConfigChange: () => undefined,
      ...legacySelection,
    }))
    expect(html).not.toContain('id="think-model"')
    expect(html).not.toContain('思考模式模型')
    expect(html).not.toContain('obsolete-think-model')
    expect(html).toContain('回答参数')
    expect(html).toContain('聊天界面')
  })

  it('does not advertise a second model in the overview', () => {
    const html = renderToStaticMarkup(createElement(GeneralSettings, {
      config, ...{ thinkModel: 'obsolete-think-model' },
    }))
    expect(html).not.toContain('obsolete-think-model')
    expect(html).not.toContain('思考模型')
    expect(html).toContain('跟随聊天模型')
  })

  it.each(['fast', 'think'] as const)('renders one accessible switch in %s mode', (mode) => {
    const html = renderToStaticMarkup(createElement(ChatArea, {
      config, chatMode: mode, ...legacySelection, sidebarOpen: true,
      activeConversation: {
        id: 'switch-test', title: '切换测试', knowledgeBaseId: '', documentId: '',
        scopeVersion: 0, messages: [], createdAt: '', updatedAt: '',
      },
      selectedKnowledgeBase: null, selectedDocument: null, isLoading: false,
      isGlobalGenerating: false, generatingConversationTitle: '', enforceSingleFlight: true,
      onChatModeChange: () => undefined, contentMode: 'default', supportsFullTableMode: false,
      onContentModeChange: () => undefined, onSendMessage: async () => true,
      onClearConversation: () => undefined,
    }))
    expect((html.match(/role="switch"/g) ?? []).length).toBe(1)
    expect(html).toContain('aria-label="思考模式"')
    expect(html).toContain('aria-checked="' + (mode === 'think') + '"')
    expect(html).not.toContain('aria-haspopup="menu"')
    expect(html).not.toContain('obsolete-')
    expect(html).toContain('configured-chat-model')
  })
})
