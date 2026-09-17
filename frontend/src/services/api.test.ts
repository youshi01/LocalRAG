import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest'
import {
	extractErrorMessage,
	fetchAvailableModels,
	normalizeConversation,
	normalizeKnowledgeBase,
	probeModel,
	serializeConversation,
} from './api'
import type { BackendConversation, BackendKnowledgeBase } from './api'

afterEach(() => {
	vi.unstubAllGlobals()
})

describe('model interface requests', () => {
	it('posts the draft endpoint to fetch available models', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			success: true,
			provider: 'ollama',
			type: 'chat',
			models: [{ id: 'qwen3.5:9b', name: 'qwen3.5:9b', type: 'chat', owned_by: 'library' }],
			latency_ms: 24,
		}), { status: 200, headers: { 'Content-Type': 'application/json' } }))
		vi.stubGlobal('fetch', fetchMock)

		const response = await fetchAvailableModels({
			provider: 'ollama', baseUrl: 'http://localhost:11434', apiKey: '',
		}, 'chat')

		expect(fetchMock).toHaveBeenCalledWith('/api/config/models', expect.objectContaining({ method: 'POST' }))
		expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toEqual({
			type: 'chat', provider: 'ollama', baseUrl: 'http://localhost:11434', apiKey: '',
			apiKeyConfigured: false, clearApiKey: false,
		})
		expect(response.models[0].id).toBe('qwen3.5:9b')
		expect(response.models[0].owned_by).toBe('library')
		expectTypeOf(response.models[0].owned_by).toEqualTypeOf<string>()
	})

	it('posts the selected model to probe it', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			success: true, type: 'embedding', provider: 'ollama', model: 'nomic-embed-text',
			latency_ms: 214, vector_size: 768, expected_vector_size: 768,
			dimension_match: true,
		}), { status: 200, headers: { 'Content-Type': 'application/json' } }))
		vi.stubGlobal('fetch', fetchMock)

		const response = await probeModel({
			provider: 'ollama', baseUrl: 'http://localhost:11434', model: 'nomic-embed-text', apiKey: '',
		}, 'embedding')

		expect(fetchMock).toHaveBeenCalledWith('/api/config/models/probe', expect.objectContaining({ method: 'POST' }))
		expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toEqual({
			type: 'embedding', provider: 'ollama', baseUrl: 'http://localhost:11434',
			model: 'nomic-embed-text', apiKey: '', apiKeyConfigured: false, clearApiKey: false,
		})
		expect(response.dimension_match).toBe(true)
	})

	it('posts explicit credential-clear intent for model discovery', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			success: true, provider: 'openai-compatible', type: 'chat', models: [], latency_ms: 3,
		}), { status: 200, headers: { 'Content-Type': 'application/json' } }))
		vi.stubGlobal('fetch', fetchMock)

		await fetchAvailableModels({
			provider: 'openai-compatible', baseUrl: 'https://provider.example/v1', apiKey: '',
			apiKeyConfigured: true, clearApiKey: true,
		}, 'chat')

		expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toMatchObject({
			apiKey: '', apiKeyConfigured: true, clearApiKey: true,
		})
	})

	it('posts chat probe temperature without UI-only fields', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			success: true, type: 'chat', provider: 'ollama', model: 'qwen3.5:9b',
			latency_ms: 24,
		}), { status: 200, headers: { 'Content-Type': 'application/json' } }))
		vi.stubGlobal('fetch', fetchMock)

		await probeModel({
			provider: 'ollama', baseUrl: 'http://localhost:11434', model: 'qwen3.5:9b', apiKey: '',
			temperature: 0.2, knowledgeTemperature: 0.1, contextMessageLimit: 10,
			apiKeyConfigured: true, clearApiKey: false,
		}, 'chat')

		const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
		expect(body).toEqual({
			type: 'chat', provider: 'ollama', baseUrl: 'http://localhost:11434',
			model: 'qwen3.5:9b', apiKey: '', apiKeyConfigured: true, clearApiKey: false, temperature: 0.2,
		})
		expect(body).not.toHaveProperty('knowledgeTemperature')
		expect(body).not.toHaveProperty('contextMessageLimit')
	})

	it('preserves an explicit null dimension match from an embedding probe', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			success: false, type: 'embedding', provider: 'ollama', model: 'nomic-embed-text',
			latency_ms: 214, vector_size: 768, expected_vector_size: 768,
			dimension_match: null,
		}), { status: 200, headers: { 'Content-Type': 'application/json' } }))
		vi.stubGlobal('fetch', fetchMock)

		const response = await probeModel({
			provider: 'ollama', baseUrl: 'http://localhost:11434', model: 'nomic-embed-text', apiKey: '',
		}, 'embedding')

		expect(response.dimension_match).toBeNull()
		expectTypeOf(response.dimension_match).toEqualTypeOf<boolean | null>()
	})
})

describe('normalizeConversation', () => {
  it('filters only legacy operational assistant messages', () => {
    const conversation: BackendConversation = {
      id: 'conversation-1',
      title: '测试会话',
      knowledgeBaseId: '',
      documentId: '',
      createdAt: '2026-07-27T00:00:00Z',
      updatedAt: '2026-07-27T00:00:03Z',
      messages: [
        {
          id: 'legacy-welcome',
          role: 'assistant',
          content:
            '你好，我是 LocalRAG 助手。你可以先选择知识库，或者进一步选中某个文档后再提问。',
          createdAt: '2026-07-27T00:00:00Z',
        },
        {
          id: 'legacy-degraded',
          role: 'assistant',
          content: '⚠️ AI 模型调用已降级\n\n模型超时',
          createdAt: '2026-07-27T00:00:01Z',
        },
        {
          id: 'real-answer',
          role: 'assistant',
          content: '文档介绍了系统的降级设计，但当前回答来自模型。',
          createdAt: '2026-07-27T00:00:02Z',
        },
        {
          id: 'user-message',
          role: 'user',
          content: '继续说明。',
          createdAt: '2026-07-27T00:00:03Z',
        },
      ],
    }

    const normalized = normalizeConversation(conversation)
    expect(normalized.scopeVersion).toBe(0)
    expect(normalized.messages.map((message) => message.id)).toEqual([
      'real-answer',
      'user-message',
    ])
  })

  it('removes legacy tool-only citation sources from stored conversations', () => {
    const conversation: BackendConversation = {
      id: 'conversation-2',
      title: '历史会话',
      knowledgeBaseId: 'kb-1',
      documentId: '',
      createdAt: '2026-07-27T00:00:00Z',
      updatedAt: '2026-07-27T00:00:01Z',
      messages: [{
        id: 'answer-1',
        role: 'assistant',
        content: '模型回答',
        createdAt: '2026-07-27T00:00:01Z',
        metadata: {
          sources: [{ toolName: 'search_knowledge_base' }],
        },
      }],
    }

    expect(normalizeConversation(conversation).messages[0].metadata).toBeUndefined()
  })

  it('preserves the conversation knowledge scope when normalizing and saving', () => {
    const backendConversation: BackendConversation = {
      id: 'conversation-scoped',
      title: '知识库会话',
      knowledgeBaseId: 'kb-school',
      documentId: 'doc-school',
      scopeVersion: 1,
      createdAt: '2026-07-27T00:00:00Z',
      updatedAt: '2026-07-27T00:00:01Z',
      messages: [{
        id: 'message-1',
        role: 'user',
        content: '详细介绍',
        createdAt: '2026-07-27T00:00:01Z',
      }],
    }

    const conversation = normalizeConversation(backendConversation)
    expect(conversation.knowledgeBaseId).toBe('kb-school')
    expect(conversation.documentId).toBe('doc-school')
    expect(conversation.scopeVersion).toBe(1)
    expect(serializeConversation(conversation)).toMatchObject({
      knowledgeBaseId: 'kb-school',
      documentId: 'doc-school',
    })
  })
})

describe('extractErrorMessage', () => {
  it('returns a readable message for an nginx 413 html response', async () => {
    const response = new Response('<html>Request Entity Too Large</html>', {
      status: 413,
      statusText: 'Request Entity Too Large',
    })

    await expect(extractErrorMessage(response)).resolves.toBe(
      '文档超过服务器允许的上传大小，请减小文件后重试',
    )
  })

  it('preserves the backend upload limit message', async () => {
    const response = new Response(JSON.stringify({
      error: 'uploaded file is too large, max size is 25.0 MiB',
    }), {
      status: 413,
      headers: { 'Content-Type': 'application/json' },
    })

    await expect(extractErrorMessage(response)).resolves.toBe(
      'uploaded file is too large, max size is 25.0 MiB',
    )
  })
})

describe('normalizeKnowledgeBase', () => {
  it('keeps governance metadata while tolerating legacy payloads', () => {
    const backendKnowledgeBase: BackendKnowledgeBase = {
      id: 'kb-1',
      name: '产品文档',
      description: '内部资料',
      tags: ['产品', '内部'],
      documents: [],
      createdAt: '2026-08-22T00:00:00Z',
      updatedAt: '2026-08-22T00:01:00Z',
      currentIndexVersion: 2,
    }

    expect(normalizeKnowledgeBase(backendKnowledgeBase)).toMatchObject({
      tags: ['产品', '内部'],
      updatedAt: '2026-08-22T00:01:00Z',
      currentIndexVersion: 2,
    })
    expect(normalizeKnowledgeBase({ ...backendKnowledgeBase, tags: undefined }).tags).toEqual([])
  })
})
