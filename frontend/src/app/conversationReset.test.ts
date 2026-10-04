import { afterEach, describe, expect, it, vi } from 'vitest'
import { createEmptyConversation } from './appHelpers'
import { ConversationOperationGuard, clearConversationWorkspace, getClearAllConversationState, loadCurrentConversationData } from './conversationReset'
import { fetchConversationDetail, fetchInitialAppData } from '../services/api'

const remoteConversation = { ...createEmptyConversation('kb-fixture'), id: 'fixture-remote', localOnly: false }
const jsonResponse = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
  status, headers: { 'Content-Type': 'application/json' },
})

afterEach(() => vi.unstubAllGlobals())

describe('clear-all eligibility for the entire, unfiltered collection', () => {
  it('counts remote summaries and local drafts rather than just loaded messages', () => {
    const state = getClearAllConversationState([remoteConversation, createEmptyConversation(), {
      ...remoteConversation, id: 'fixture-second', title: '搜索不可见的历史',
    }])
    expect(state.totalCount).toBe(3)
    expect(state.meaningfulCount).toBe(2)
    expect(state.canClear).toBe(true)
  })

  it.each([
    { conversations: [] },
    { conversations: [createEmptyConversation()] },
    { conversations: [createEmptyConversation(), createEmptyConversation()] },
  ])('does not treat empty auto-created local drafts as meaningful history', ({ conversations }) => {
    expect(getClearAllConversationState(conversations).canClear).toBe(false)
  })

  it('allows clearing a local draft containing messages', () => {
    expect(getClearAllConversationState([{
      ...createEmptyConversation(),
      messages: [{ id: 'fixture-message', role: 'user', content: '测试消息', timestamp: '' }],
    }]).canClear).toBe(true)
  })

  it('allows clearing an explicitly renamed local draft', () => {
    expect(getClearAllConversationState([{ ...createEmptyConversation(), title: '已命名的草稿' }]).canClear).toBe(true)
  })

  it.each([{ isGenerating: true }, { isClearing: true }, { isMutating: true }])(
    'blocks clear-all while conversation work is in flight: %j', (flags) => {
      expect(getClearAllConversationState([remoteConversation], flags).canClear).toBe(false)
    },
  )
})

describe('conversation operation guard', () => {
  it('does not let a pending save cross a clear-all boundary', () => {
    const guard = new ConversationOperationGuard()
    const release = guard.beginMutation()
    expect(release).not.toBeNull()
    expect(guard.isMutating).toBe(true)
    expect(guard.beginClear()).toBe(false)
    release?.()
    release?.()
    expect(guard.isMutating).toBe(false)
    expect(guard.beginClear()).toBe(true)
    expect(guard.beginMutation()).toBeNull()
  })

  it('keeps clearing blocked until every overlapping mutation finishes', () => {
    const guard = new ConversationOperationGuard()
    const first = guard.beginMutation()
    const second = guard.beginMutation()
    first?.()
    first?.()
    expect(guard.isMutating).toBe(true)
    expect(guard.beginClear()).toBe(false)
    second?.()
    expect(guard.beginClear()).toBe(true)
  })

  it('invalidates late detail/bootstrap responses only after successful deletion', () => {
    const guard = new ConversationOperationGuard()
    const oldRevision = guard.revision
    expect(guard.beginClear()).toBe(true)
    expect(guard.isCurrent(oldRevision)).toBe(false)
    guard.finishClear(false)
    expect(guard.isCurrent(oldRevision)).toBe(true)
    guard.beginClear()
    guard.finishClear(true)
    expect(guard.isCurrent(oldRevision)).toBe(false)
    expect(guard.isCurrent(guard.revision)).toBe(true)
  })
})

describe('clear-all reset transaction (mock fetch only)', () => {
  it('waits for confirmed API success then drops every old conversation and navigation reference', async () => {
    let resolveRequest!: (response: Response) => void
    const fetchMock = vi.fn().mockImplementation(() => new Promise<Response>((resolve) => { resolveRequest = resolve }))
    vi.stubGlobal('fetch', fetchMock)
    const guard = new ConversationOperationGuard()
    const oldRevision = guard.revision
    const onCleared = vi.fn()
    const pending = clearConversationWorkspace(guard, { knowledgeBaseId: 'kb-kept', documentId: 'doc-kept' }, onCleared)

    expect(onCleared).not.toHaveBeenCalled()
    expect(guard.isClearing).toBe(true)
    expect(await clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, onCleared)).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    resolveRequest(jsonResponse({ deletedCount: 2 }))
    expect(await pending).toEqual({ deletedCount: 2 })

    expect(onCleared).toHaveBeenCalledTimes(1)
    const [snapshot] = onCleared.mock.calls[0]
    expect(snapshot.conversations).toHaveLength(1)
    expect(snapshot.conversations[0]).toMatchObject({
      title: '新的对话', localOnly: true, messages: [], scopeVersion: 1,
      knowledgeBaseId: 'kb-kept', documentId: 'doc-kept',
    })
    expect(snapshot.conversations[0].id).not.toBe('fixture-remote')
    expect(snapshot.activeConversationId).toBe(snapshot.conversations[0].id)
    expect(snapshot.streamingConversationId).toBeNull()
    expect(snapshot.citationNavigationTarget).toBeNull()
    expect(snapshot.resetKey).toBe(1)
    expect(guard.isCurrent(oldRevision)).toBe(false)
    expect(guard.isClearing).toBe(false)
  })

  it.each([400, 409, 500])('keeps all local state and old references intact after HTTP %i', async (status) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: '测试失败' }, status)))
    const guard = new ConversationOperationGuard()
    const originalRevision = guard.revision
    const onCleared = vi.fn()
    await expect(clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, onCleared)).rejects.toThrow()
    expect(onCleared).not.toHaveBeenCalled()
    expect(guard.revision).toBe(originalRevision)
    expect(guard.isClearing).toBe(false)
    expect(guard.beginMutation()).not.toBeNull()
  })

  it('does not clear local state on a network failure or malformed receipt and permits retry', async () => {
    const fetchMock = vi.fn().mockRejectedValueOnce(new Error('离线'))
      .mockResolvedValueOnce(jsonResponse({}))
      .mockResolvedValueOnce(jsonResponse({ deletedCount: 0 }))
    vi.stubGlobal('fetch', fetchMock)
    const guard = new ConversationOperationGuard()
    const onCleared = vi.fn()
    const clear = () => clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, onCleared)
    await expect(clear()).rejects.toThrow('离线')
    await expect(clear()).rejects.toThrow('无效的删除数量')
    expect(onCleared).not.toHaveBeenCalled()
    expect(await clear()).toEqual({ deletedCount: 0 })
    expect(onCleared).toHaveBeenCalledTimes(1)
  })

  it('does not issue a deletion while another conversation mutation is pending', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const guard = new ConversationOperationGuard()
    guard.beginMutation()
    const onCleared = vi.fn()
    expect(await clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, onCleared)).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(onCleared).not.toHaveBeenCalled()
  })
})

describe('in-flight detail and history loaders', () => {
  it('discards a late detail response after clear-all succeeds', async () => {
    let resolveDetail!: (response: Response) => void
    vi.stubGlobal('fetch', vi.fn().mockImplementation((_url: string, init: RequestInit) => (
      init.method === 'DELETE'
        ? Promise.resolve(jsonResponse({ deletedCount: 1 }))
        : new Promise<Response>((resolve) => { resolveDetail = resolve })
    )))
    const guard = new ConversationOperationGuard()
    const pendingDetail = loadCurrentConversationData(guard, () => fetchConversationDetail('fixture-remote'))
    await clearConversationWorkspace(guard, { knowledgeBaseId: 'kb-kept', documentId: 'doc-kept' }, () => undefined)
    resolveDetail(jsonResponse({ ...remoteConversation, messages: [] }))
    expect(await pendingDetail).toBeNull()
  })

  it('discards a late history list before bootstrap can hydrate old conversation summaries', async () => {
    let resolveHistory!: (response: Response) => void
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string, init: RequestInit) => {
      if (init.method === 'DELETE') return Promise.resolve(jsonResponse({ deletedCount: 1 }))
      if (url === '/api/conversations') return new Promise<Response>((resolve) => { resolveHistory = resolve })
      return Promise.resolve(jsonResponse(url === '/api/config' ? {} : { items: [] }))
    }))
    const guard = new ConversationOperationGuard()
    const pendingHistory = loadCurrentConversationData(guard, fetchInitialAppData)
    await clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, () => undefined)
    resolveHistory(jsonResponse({ items: [{ ...remoteConversation, messageCount: 2 }] }))
    expect(await pendingHistory).toBeNull()
  })

  it('does not start a new detail request while clearing', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const guard = new ConversationOperationGuard()
    guard.beginClear()
    expect(await loadCurrentConversationData(guard, () => fetchConversationDetail('fixture-remote'))).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('suppresses errors from expired loads but still exposes a current loader failure', async () => {
    let rejectDetail!: (error: Error) => void
    vi.stubGlobal('fetch', vi.fn().mockImplementation((_url: string, init: RequestInit) => (
      init.method === 'DELETE'
        ? Promise.resolve(jsonResponse({ deletedCount: 1 }))
        : new Promise<Response>((_resolve, reject) => { rejectDetail = reject })
    )))
    const guard = new ConversationOperationGuard()
    const pendingDetail = loadCurrentConversationData(guard, () => fetchConversationDetail('fixture-remote'))
    await clearConversationWorkspace(guard, { knowledgeBaseId: '', documentId: '' }, () => undefined)
    rejectDetail(new Error('已删除的会话不存在'))
    expect(await pendingDetail).toBeNull()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('当前请求离线')))
    await expect(loadCurrentConversationData(guard, () => fetchConversationDetail('fixture-new'))).rejects.toThrow('当前请求离线')
  })

  it('leaves unrelated stored theme, model and sidebar preferences untouched', async () => {
    const entries = new Map([
      ['theme', 'dark'], ['localrag-sidebar-open', 'false'], ['fixture-model-preference', 'fixture-model'],
    ])
    const storage = {
      getItem: (key: string) => entries.get(key) ?? null,
      setItem: (key: string, value: string) => entries.set(key, value),
      removeItem: vi.fn((key: string) => entries.delete(key)),
      clear: vi.fn(() => entries.clear()),
    }
    vi.stubGlobal('localStorage', storage)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ deletedCount: 1 })))
    await clearConversationWorkspace(new ConversationOperationGuard(), { knowledgeBaseId: '', documentId: '' }, () => undefined)
    expect([...entries]).toEqual([
      ['theme', 'dark'], ['localrag-sidebar-open', 'false'], ['fixture-model-preference', 'fixture-model'],
    ])
    expect(storage.clear).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })
})
