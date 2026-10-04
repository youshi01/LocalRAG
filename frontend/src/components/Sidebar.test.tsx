import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { createEmptyConversation } from '../app/appHelpers'
import Sidebar from './Sidebar'

const renderSidebar = (overrides = {}) => renderToStaticMarkup(createElement(Sidebar, {
  isOpen: true, onToggle: () => undefined, activeWorkspace: 'chat',
  onChangeWorkspace: () => undefined, conversations: [{
    ...createEmptyConversation('kb-fixture'), id: 'fixture-remote', localOnly: false,
  }], activeConversationId: 'fixture-remote', onSelectConversation: () => undefined,
  onCreateConversation: () => undefined, onRenameConversation: () => undefined,
  onDeleteConversation: () => undefined,
  ...{ onClearAllConversations: () => undefined, isGenerating: false, isClearingConversations: false },
  ...overrides,
}))

const clearButton = (html: string) => html.match(/<button\b[^>]*aria-label="清空本项目全部会话"[^>]*>/)?.[0] ?? ''

describe('Sidebar clear-all action', () => {
  it('places an accessible clear-all button next to the new-conversation button', () => {
    const html = renderSidebar()
    const header = html.match(/<header\b[\s\S]*?<\/header>/)?.[0] ?? ''
    expect(header).toContain('aria-label="新建会话"')
    expect(clearButton(header)).not.toBe('')
    expect(clearButton(header)).not.toContain('disabled')
  })

  it.each([
    { isGenerating: true },
    { isClearingConversations: true },
    { isConversationMutating: true },
    { conversations: [] },
    { conversations: [createEmptyConversation('kb-fixture')] },
  ])('disables clear-all for an unsafe or empty state: %j', (overrides) => {
    const button = clearButton(renderSidebar(overrides))
    expect(button).not.toBe('')
    expect(button).toContain('disabled')
  })

  it('allows deletion of an unloaded remote history item with an empty local message array', () => {
    const button = clearButton(renderSidebar())
    expect(button).not.toBe('')
    expect(button).not.toContain('disabled')
  })

  it('disables creation and marks the collection busy while clearing', () => {
    const html = renderSidebar({ isClearingConversations: true })
    expect(html.match(/<button\b[^>]*aria-label="新建会话"[^>]*>/)?.[0]).toContain('disabled')
    expect(html).toContain('aria-busy="true"')
  })
})
