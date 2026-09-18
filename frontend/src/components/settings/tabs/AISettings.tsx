import React, { useState } from 'react'
import type { AppConfig, ChatConfig, ChatModeSettings, EmbeddingConfig } from '../../../App'
import AppIcon from '../../common/AppIcon'
import { ModelConfigTest } from '../ModelConfigProbe'
import { getEmbeddingGuide } from '../embeddingGuide'

interface AISettingsProps {
  config: AppConfig
  onChatConfigChange: <K extends keyof ChatConfig>(key: K, value: ChatConfig[K]) => void
  onEmbeddingConfigChange: <K extends keyof EmbeddingConfig>(
    key: K,
    value: EmbeddingConfig[K],
  ) => void
  chatModeSettings: ChatModeSettings
  onThinkModelChange: (value: string) => void
}

type ModelSection = 'chat' | 'embedding'

const AISettings: React.FC<AISettingsProps> = ({
  config,
  onChatConfigChange,
  onEmbeddingConfigChange,
  chatModeSettings,
  onThinkModelChange,
}) => {
  const [activeSection, setActiveSection] = useState<ModelSection>('chat')
  const embeddingGuide = getEmbeddingGuide(config.embedding.provider)

  return (
    <div className="settings-tab-content settings-models-page">
      <div className="settings-subnav" aria-label="模型类型" role="tablist">
        <button
          aria-controls="settings-model-panel-chat"
          aria-selected={activeSection === 'chat'}
          className={activeSection === 'chat' ? 'active' : ''}
          id="settings-model-tab-chat"
          onClick={() => setActiveSection('chat')}
          role="tab"
          type="button"
        >
          <AppIcon name="message" size={16} />
          <span>聊天模型</span>
        </button>
        <button
          aria-controls="settings-model-panel-embedding"
          aria-selected={activeSection === 'embedding'}
          className={activeSection === 'embedding' ? 'active' : ''}
          id="settings-model-tab-embedding"
          onClick={() => setActiveSection('embedding')}
          role="tab"
          type="button"
        >
          <AppIcon name="database" size={16} />
          <span>Embedding</span>
        </button>
      </div>

      {activeSection === 'chat' ? (
        <section
          aria-labelledby="settings-model-tab-chat"
          className="settings-config-panel"
          id="settings-model-panel-chat"
          role="tabpanel"
        >
          <section className="settings-form-section">
            <header>
              <h4>连接</h4>
              <p>模型服务地址和身份凭据。</p>
            </header>
            <div className="settings-form-grid settings-form-grid-dense">
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="chat-provider">Provider</label>
                <select
                  id="chat-provider"
                  value={config.chat.provider}
                  onChange={(event) => onChatConfigChange('provider', event.target.value as ChatConfig['provider'])}
                >
                  <option value="ollama">Ollama</option>
                  <option value="openai-compatible">OpenAI Compatible</option>
                </select>
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="chat-base-url">Base URL</label>
                <input
                  id="chat-base-url"
                  type="text"
                  value={config.chat.baseUrl}
                  onChange={(event) => onChatConfigChange('baseUrl', event.target.value)}
                  placeholder={config.chat.provider === 'ollama' ? 'http://localhost:11434' : 'http://localhost:11434/v1'}
                />
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="chat-model">Model</label>
                <input
                  id="chat-model"
                  type="text"
                  value={config.chat.model}
                  onChange={(event) => onChatConfigChange('model', event.target.value)}
                  placeholder="llama3.2"
                />
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="chat-api-key">API Key</label>
                <div className="settings-secret-input-row">
                  <input
                    id="chat-api-key"
                    type="password"
                    value={config.chat.apiKey}
                    onChange={(event) => onChatConfigChange('apiKey', event.target.value)}
                    placeholder={config.chat.apiKeyConfigured ? '已配置，输入新密钥覆盖' : '选填'}
                  />
                  {(config.chat.apiKeyConfigured || config.chat.apiKey) && (
                    <button
                      type="button"
                      className="settings-action-btn"
                      onClick={() => onChatConfigChange('clearApiKey', true)}
                    >
                      清除
                    </button>
                  )}
                </div>
                {config.chat.apiKeyConfigured && !config.chat.apiKey && (
                  <small>密钥已保存在后端，页面不会显示明文。</small>
                )}
              </div>
            </div>
          </section>

          <section className="settings-form-section settings-model-parameters">
            <header>
              <h4>生成与推理</h4>
              <p>温度、会话上下文和思考模式模型。</p>
            </header>
            <div className="settings-temperature-grid">
              <div className="settings-temperature-control">
                <label className="settings-form-label settings-form-label-inline" htmlFor="chat-temperature">
                  <span>普通聊天</span>
                  <strong>{config.chat.temperature.toFixed(1)}</strong>
                </label>
                <small>控制未使用知识库时回答的自由度。</small>
                <input
                  id="chat-temperature"
                  type="range"
                  min="0"
                  max="1"
                  step="0.1"
                  value={config.chat.temperature}
                  onChange={(event) => onChatConfigChange('temperature', Number(event.target.value))}
                />
              </div>
              <div className="settings-temperature-control">
                <label className="settings-form-label settings-form-label-inline" htmlFor="knowledge-temperature">
                  <span>知识库问答</span>
                  <strong>{config.chat.knowledgeTemperature.toFixed(1)}</strong>
                </label>
                <small>较低温度更适合引用和事实回答。</small>
                <input
                  id="knowledge-temperature"
                  type="range"
                  min="0.1"
                  max="0.5"
                  step="0.1"
                  value={config.chat.knowledgeTemperature}
                  onChange={(event) => onChatConfigChange('knowledgeTemperature', Number(event.target.value))}
                />
              </div>
            </div>
            <div className="settings-form-grid settings-form-grid-dense settings-model-context-grid">
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="chat-context-limit">上下文消息数量</label>
                <input
                  id="chat-context-limit"
                  type="number"
                  min="1"
                  max="100"
                  value={config.chat.contextMessageLimit}
                  onChange={(event) => onChatConfigChange('contextMessageLimit', Number(event.target.value))}
                />
                <small>每次发送给模型的最近消息条数，范围 1-100。</small>
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="think-model">思考模式模型</label>
                <input
                  id="think-model"
                  type="text"
                  value={chatModeSettings.thinkModel}
                  onChange={(event) => onThinkModelChange(event.target.value)}
                  placeholder="deepseek-r1:8b"
                />
                <small>留空时使用聊天模型。</small>
              </div>
            </div>
          </section>

          <div className="settings-test-row">
            <div>
              <strong>获取模型</strong>
              <span>读取当前协议提供的模型候选，不会下载模型。</span>
            </div>
            <ModelConfigTest
              type="chat"
              provider={config.chat.provider}
              baseUrl={config.chat.baseUrl}
              modelName={config.chat.model}
              apiKey={config.chat.apiKey}
              apiKeyConfigured={config.chat.apiKeyConfigured}
              clearApiKey={config.chat.clearApiKey}
              temperature={config.chat.temperature}
              showProbe={false}
              onModelChange={(value) => onChatConfigChange('model', value)}
            />
          </div>
        </section>
      ) : (
        <section
          aria-labelledby="settings-model-tab-embedding"
          className="settings-config-panel"
          id="settings-model-panel-embedding"
          role="tabpanel"
        >
          <section className="settings-form-section">
            <header>
              <h4>连接</h4>
              <p>Embedding 服务地址、模型和凭据。Embedding 可以来自公网、公司内网或本机服务。</p>
            </header>
            <div className="settings-form-grid settings-form-grid-dense">
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="embedding-provider">Provider</label>
                <select
                  id="embedding-provider"
                  value={config.embedding.provider}
                  onChange={(event) => onEmbeddingConfigChange('provider', event.target.value as EmbeddingConfig['provider'])}
                >
                  <option value="ollama">Ollama</option>
                  <option value="openai-compatible">OpenAI Compatible</option>
                </select>
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="embedding-base-url">Base URL</label>
                <input
                  id="embedding-base-url"
                  type="text"
                  value={config.embedding.baseUrl}
                  onChange={(event) => onEmbeddingConfigChange('baseUrl', event.target.value)}
                  placeholder={config.embedding.provider === 'ollama' ? 'http://localhost:11434' : 'http://localhost:11434/v1'}
                />
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="embedding-model">Model</label>
                <input
                  id="embedding-model"
                  type="text"
                  value={config.embedding.model}
                  onChange={(event) => onEmbeddingConfigChange('model', event.target.value)}
                  placeholder="nomic-embed-text"
                />
              </div>
              <div className="settings-form-group">
                <label className="settings-form-label" htmlFor="embedding-api-key">API Key</label>
                <div className="settings-secret-input-row">
                  <input
                    id="embedding-api-key"
                    type="password"
                    value={config.embedding.apiKey}
                    onChange={(event) => onEmbeddingConfigChange('apiKey', event.target.value)}
                    placeholder={config.embedding.apiKeyConfigured ? '已配置，输入新密钥覆盖' : '选填'}
                  />
                  {(config.embedding.apiKeyConfigured || config.embedding.apiKey) && (
                    <button
                      type="button"
                      className="settings-action-btn"
                      onClick={() => onEmbeddingConfigChange('clearApiKey', true)}
                    >
                      清除
                    </button>
                  )}
                </div>
                {config.embedding.apiKeyConfigured && !config.embedding.apiKey && (
                  <small>密钥已保存在后端，页面不会显示明文。</small>
                )}
              </div>
            </div>
          </section>

          <div className="settings-test-row">
            <div>
              <strong>获取模型</strong>
              <span>读取当前协议提供的模型候选，不会下载模型。</span>
            </div>
            <ModelConfigTest
              type="embedding"
              provider={config.embedding.provider}
              baseUrl={config.embedding.baseUrl}
              modelName={config.embedding.model}
              apiKey={config.embedding.apiKey}
              apiKeyConfigured={config.embedding.apiKeyConfigured}
              clearApiKey={config.embedding.clearApiKey}
              showProbe={false}
              onModelChange={(value) => onEmbeddingConfigChange('model', value)}
            />
          </div>

          <aside className="settings-embedding-guide" aria-label="Embedding 配置说明">
            <div className="settings-embedding-guide-header">
              <div>
                <h4>Embedding 能力说明</h4>
                <span>{embeddingGuide.protocol} · {embeddingGuide.endpoint}</span>
              </div>
              <span className="settings-status-pill neutral">按协议探测</span>
            </div>
            <p className="settings-embedding-guide-intro">{embeddingGuide.whatIsEmbedding}</p>
            <div className="settings-embedding-guide-recommendations">
              <span className="settings-embedding-guide-label">模型推荐方向</span>
              <ul>
                {embeddingGuide.recommendations.map((item) => (
                  <li key={item.name}>
                    <strong>{item.name}</strong>
                    <span>{item.description}</span>
                  </li>
                ))}
              </ul>
              <small>推荐仅用于选型参考，最终以服务返回的 Embedding 能力和向量维度为准。</small>
            </div>
            <dl className="settings-embedding-guide-grid">
              <div>
                <dt className="settings-embedding-guide-label">可用平台</dt>
                <dd>{embeddingGuide.platforms}</dd>
              </div>
              <div>
                <dt className="settings-embedding-guide-label">Base URL 示例</dt>
                <dd><code>{embeddingGuide.baseUrlExample}</code>，不要填写完整的 embeddings 路径。</dd>
              </div>
              <div>
                <dt className="settings-embedding-guide-label">模型选择</dt>
                <dd>{embeddingGuide.modelRule}</dd>
              </div>
              <div>
                <dt className="settings-embedding-guide-label">索引要求</dt>
                <dd>{embeddingGuide.dimensionRule}</dd>
              </div>
            </dl>
            <p className="settings-embedding-guide-note">{embeddingGuide.serverNote}</p>
          </aside>

        </section>
      )}
    </div>
  )
}

export default AISettings
