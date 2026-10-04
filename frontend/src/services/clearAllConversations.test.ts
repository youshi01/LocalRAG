import { afterEach, describe, expect, it, vi } from 'vitest'
import * as api from './api'

const jsonResponse = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
  status,
  headers: { 'Content-Type': 'application/json' },
})

afterEach(() => vi.unstubAllGlobals())

describe('clear all conversations API (mock fetch only)', () => {
  it('sends one explicitly confirmed collection DELETE with the existing CSRF cookie', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ message: 'all conversations cleared', deletedCount: 3 }))
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('document', { cookie: 'localrag_csrf=fixture%2Fcsrf' })

    expect(api).toHaveProperty('clearAllConversations', expect.any(Function))
    const result = await api.clearAllConversations()

    expect(result).toEqual({ message: 'all conversations cleared', deletedCount: 3 })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe(`${api.API_BASE_PATH}/api/conversations`)
    expect(init.method).toBe('DELETE')
    expect(init.credentials).toBe('same-origin')
    expect(JSON.parse(init.body as string)).toEqual({ confirm: true })
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(new Headers(init.headers).get(api.CSRF_HEADER_NAME)).toBe('fixture/csrf')
  })

  it('retains legacy CSRF cookie support', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ deletedCount: 0 }))
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('document', { cookie: 'ai_localbase_csrf=legacy-fixture' })

    await api.clearAllConversations()
    expect(new Headers(fetchMock.mock.calls[0][1].headers).get(api.CSRF_HEADER_NAME)).toBe('legacy-fixture')
  })

  it('uses existing structured error handling for a rejected confirmation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      error: { code: 'confirmation_required', message: '请明确确认清空所有会话。' },
    }, 400)))

    await expect(api.clearAllConversations()).rejects.toThrow('请明确确认清空所有会话。')
  })

  it('explains a server-side active-chat conflict in Chinese even if the server uses an English error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: 'active chat request' }, 409)))

    await expect(api.clearAllConversations()).rejects.toThrow('当前有会话正在生成或修改，请等待完成后再清空本项目全部会话。')
  })

  it('preserves the backend Chinese conflict explanation for a mutation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
      error: { message: '当前有会话操作正在进行，请稍后再清空全部会话。' },
    }, 409)))
    await expect(api.clearAllConversations()).rejects.toThrow('当前有会话操作正在进行，请稍后再清空全部会话。')
  })

  it('propagates a network failure instead of reporting success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('网络连接失败')))
    await expect(api.clearAllConversations()).rejects.toThrow('网络连接失败')
  })

  it.each([{}, { deletedCount: -1 }, { deletedCount: '3' }, { deletedCount: 1.5 }])(
    'rejects a malformed deletion receipt: %j', async (receipt) => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(receipt)))
      await expect(api.clearAllConversations()).rejects.toThrow('清空会话接口返回了无效的删除数量')
    },
  )

  it('does not use a browser-cached conversation list when bootstrapping after deletion', async () => {
    const fetchMock = vi.fn().mockImplementation(async (url: string) => (
      jsonResponse(url.endsWith('/config') ? {} : { items: [] })
    ))
    vi.stubGlobal('fetch', fetchMock)

    expect((await api.fetchInitialAppData()).conversations).toEqual([])
    expect(fetchMock).toHaveBeenCalledWith(`${api.API_BASE_PATH}/api/conversations`, expect.objectContaining({
      cache: 'no-store',
    }))
  })

  it('does not use a browser-cached conversation detail', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      id: 'fixture-remote', title: '测试会话', knowledgeBaseId: 'kb-fixture', documentId: '',
      scopeVersion: 1, messages: [], createdAt: '', updatedAt: '',
    }))
    vi.stubGlobal('fetch', fetchMock)

    await api.fetchConversationDetail('fixture-remote')
    expect(fetchMock).toHaveBeenCalledWith(`${api.API_BASE_PATH}/api/conversations/fixture-remote`, expect.objectContaining({
      cache: 'no-store',
    }))
  })
})
