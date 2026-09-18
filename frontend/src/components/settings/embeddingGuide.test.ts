import { describe, expect, it } from 'vitest'
import { getEmbeddingGuide } from './embeddingGuide'

describe('embedding configuration guide', () => {
  it('describes OpenAI Compatible embedding as a protocol, not a vendor lock', () => {
    const guide = getEmbeddingGuide('openai-compatible')

    expect(guide.endpoint).toBe('POST /v1/embeddings')
    expect(guide.baseUrlExample).toContain('/v1')
    expect(guide.platforms).toContain('vLLM')
    expect(guide.platforms).toContain('公司内部平台')
    expect(guide.modelRule).toContain('Embedding')
    expect(guide.modelRule).toContain('Chat')
  })

  it('explains Ollama as one protocol while keeping the same capability rules', () => {
    const guide = getEmbeddingGuide('ollama')

    expect(guide.endpoint).toBe('POST /api/embed')
    expect(guide.baseUrlExample).toBe('http://localhost:11434')
    expect(guide.dimensionRule).toContain('QDRANT_VECTOR_SIZE')
    expect(guide.dimensionRule).toContain('QDRANT_COLLECTION_PREFIX')
    expect(guide.serverNote).toContain('LocalRAG')
  })
})
