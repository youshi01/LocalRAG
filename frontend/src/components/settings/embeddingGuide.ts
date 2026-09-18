import type { EmbeddingConfig } from '../../App'

export interface EmbeddingGuide {
  whatIsEmbedding: string
  protocol: string
  endpoint: string
  baseUrlExample: string
  platforms: string
  modelRule: string
  dimensionRule: string
  serverNote: string
  recommendations: Array<{
    name: string
    description: string
  }>
}

const whatIsEmbedding = 'Embedding 模型会把文本转换成向量，供文档索引和知识库检索使用；它不负责生成聊天答案。'
const sharedModelRule = '必须选择真正的 Text Embedding 模型；普通 Chat、Instruct 或 Reasoning 模型不一定支持向量化。'
const sharedDimensionRule = '点击“探测模型”确认输出维度，并确保与 QDRANT_VECTOR_SIZE 一致；更换 Embedding 模型后应重新索引，维度变化时使用新的 QDRANT_COLLECTION_PREFIX。'
const recommendations = [
  { name: 'nomic-embed-text', description: '本地或内网通用文本向量模型，适合入门和低成本部署。' },
  { name: 'bge-m3', description: '多语言和中英文检索常用，适合公司内部知识库。' },
  { name: 'text-embedding-3-small', description: '公网 OpenAI Compatible 服务的低成本选择。' },
  { name: 'text-embedding-3-large', description: '更高质量的公网向量模型，成本和资源占用更高。' },
]

export const getEmbeddingGuide = (provider: EmbeddingConfig['provider']): EmbeddingGuide => {
  if (provider === 'ollama') {
    return {
      whatIsEmbedding,
      protocol: 'Ollama Native',
      endpoint: 'POST /api/embed',
      baseUrlExample: 'http://localhost:11434',
      platforms: 'Ollama 本机、局域网或公司内部 Ollama 服务',
      modelRule: sharedModelRule,
      dimensionRule: sharedDimensionRule,
      serverNote: 'LocalRAG 只调用已经运行的模型服务，不会替服务添加 --embeddings 启动参数。',
      recommendations,
    }
  }

  return {
    whatIsEmbedding,
    protocol: 'OpenAI Compatible',
    endpoint: 'POST /v1/embeddings',
    baseUrlExample: 'https://your-server.example.com/v1',
    platforms: 'vLLM、llama.cpp、LocalAI、Xinference、云端或公司内部平台（只要暴露兼容接口）',
    modelRule: sharedModelRule,
    dimensionRule: sharedDimensionRule,
    serverNote: '如果服务需要 --embeddings 等启动参数，请在模型服务器侧启用；LocalRAG 不负责启动远程服务。',
    recommendations,
  }
}
