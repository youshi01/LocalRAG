import type { EmbeddingConfig } from '../../App'

export interface EmbeddingGuide {
  protocol: string
  endpoint: string
  baseUrlExample: string
  platforms: string
  modelRule: string
  dimensionRule: string
  serverNote: string
}

const sharedModelRule = '必须选择真正的 Text Embedding 模型；普通 Chat、Instruct 或 Reasoning 模型不一定支持向量化。'
const sharedDimensionRule = '点击“探测模型”确认输出维度，并确保与 QDRANT_VECTOR_SIZE 一致；更换 Embedding 模型后应重新索引，维度变化时使用新的 QDRANT_COLLECTION_PREFIX。'

export const getEmbeddingGuide = (provider: EmbeddingConfig['provider']): EmbeddingGuide => {
  if (provider === 'ollama') {
    return {
      protocol: 'Ollama Native',
      endpoint: 'POST /api/embed',
      baseUrlExample: 'http://localhost:11434',
      platforms: 'Ollama 本机、局域网或公司内部 Ollama 服务',
      modelRule: sharedModelRule,
      dimensionRule: sharedDimensionRule,
      serverNote: 'LocalRAG 只调用已经运行的模型服务，不会替服务添加 --embeddings 启动参数。',
    }
  }

  return {
    protocol: 'OpenAI Compatible',
    endpoint: 'POST /v1/embeddings',
    baseUrlExample: 'https://your-server.example.com/v1',
    platforms: 'vLLM、llama.cpp、LocalAI、Xinference、云端或公司内部平台（只要暴露兼容接口）',
    modelRule: sharedModelRule,
    dimensionRule: sharedDimensionRule,
    serverNote: '如果服务需要 --embeddings 等启动参数，请在模型服务器侧启用；LocalRAG 不负责启动远程服务。',
  }
}
