import { describe, expect, it } from 'vitest'
import { buildChatRequestBody } from './chatRequest'

describe('chat request boundary', () => {
  it('serializes only the message fields accepted by the API', () => {
    const request = buildChatRequestBody({
      conversationId: 'conversation-1',
      think: false,
      knowledgeBaseId: 'kb-1',
      documentId: '',
      retrievalMode: 'dense',
      contentMode: 'full_table',
      config: {
        provider: 'ollama',
        baseUrl: 'http://localhost:11434/v1',
        model: 'llama3.2',
        apiKey: '',
        temperature: 0.7,
        knowledgeTemperature: 0.1,
        contextMessageLimit: 12,
      },
      embedding: {
        provider: 'ollama',
        baseUrl: 'http://localhost:11434/v1',
        model: 'nomic-embed-text',
        apiKey: '',
      },
      messages: [{
        id: 'message-1',
        role: 'user',
        content: '问题',
        timestamp: '2026-08-29T00:00:00Z',
        metadata: { degraded: false },
      }],
    })

    expect(request.messages).toEqual([{ role: 'user', content: '问题' }])
    expect(request.conversationId).toBe('conversation-1')
    expect(request.config.model).toBe('llama3.2')
    expect(request.contentMode).toBe('full_table')
  })
})


describe('thinking keeps the configured chat model', () => {
  it.each([false, true])('uses the same model and embedding when think=%s', (think) => {
    const input = {
      conversationId: 'same-model', model: 'obsolete-thinking-model', think,
      knowledgeBaseId: 'kb', documentId: '', retrievalMode: 'dense' as const,
      contentMode: 'default' as const,
      config: {
        provider: 'ollama' as const, baseUrl: 'http://localhost:11434',
        model: 'configured-chat-model', apiKey: '', temperature: 0.7,
        knowledgeTemperature: 0.1, contextMessageLimit: 12,
      },
      embedding: { provider: 'openai-compatible' as const, baseUrl: 'https://embeddings.example/v1', model: 'embedding-model', apiKey: '' },
      messages: [],
    }
    const request = buildChatRequestBody(input)
    expect(request.model).toBe('configured-chat-model')
    expect(request.config).toEqual(input.config)
    expect(request.think).toBe(think)
    expect(request.embedding).toEqual(input.embedding)
    expect(request.embedding).not.toHaveProperty('think')
  })
})
