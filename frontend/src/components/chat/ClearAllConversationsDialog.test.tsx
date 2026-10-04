import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import ClearAllConversationsDialog from './ClearAllConversationsDialog'

const renderDialog = (overrides = {}) => renderToStaticMarkup(createElement(ClearAllConversationsDialog, {
  open: true, conversationCount: 3, isClearing: false, canConfirm: true,
  onConfirm: () => undefined, onCancel: () => undefined, ...overrides,
}))

describe('explicit second clear-all confirmation', () => {
  it('states the total scope, irreversible effect and unaffected assets', () => {
    const html = renderDialog()
    expect(html).toContain('role="dialog"')
    expect(html).toContain('3 个会话')
    expect(html).toContain('不受搜索筛选影响')
    expect(html).toContain('本项目全部会话')
    expect(html).toContain('所有知识库范围')
    expect(html).toContain('全部消息')
    expect(html).toContain('不可恢复')
    expect(html).toContain('不可撤销')
    expect(html).toContain('知识库、文件、索引和设置均不受影响')
    expect(html).toContain('确认清空本项目全部会话')
    expect(html).toContain('取消')
  })

  it('does not render the confirmation until requested', () => {
    expect(renderDialog({ open: false })).toBe('')
  })

  it('disables confirmation if generation or another mutation makes clearing ineligible', () => {
    const html = renderDialog({ canConfirm: false })
    expect(html.match(/<button[^>]*class="confirm-btn confirm-btn--confirm"[^>]*>/)?.[0]).toContain('disabled')
  })

  it('disables confirmation and cancellation while deletion is pending', () => {
    const html = renderDialog({ isClearing: true })
    expect(html.match(/<button[^>]*class="confirm-btn confirm-btn--confirm"[^>]*>/)?.[0]).toContain('disabled')
    expect(html.match(/<button[^>]*class="confirm-btn confirm-btn--cancel"[^>]*>/)?.[0]).toContain('disabled')
    expect(html).toContain('正在清空')
  })
})
