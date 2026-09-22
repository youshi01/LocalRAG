import React, { useEffect, useRef, useState } from 'react'
import { fetchAvailableModels } from '../../services/api'
import type { ChatConfig, EmbeddingConfig } from '../../App'
import type { ModelOption } from '../../services/api'
import {
  isCurrentModelDiscoveryRequest,
  modelOptionLabel,
  resetModelOptionsKey,
} from './modelOptions'

type ModelConfigTestProps =
  | {
      type: 'chat'
      provider: ChatConfig['provider']
      baseUrl: string
      modelName: string
      apiKey: string
      apiKeyConfigured?: boolean
      clearApiKey?: boolean
      onModelChange: (value: string) => void
    }
  | {
      type: 'embedding'
      provider: EmbeddingConfig['provider']
      baseUrl: string
      modelName: string
      apiKey: string
      apiKeyConfigured?: boolean
      clearApiKey?: boolean
      onModelChange: (value: string) => void
    }

export const ModelConfigTest: React.FC<ModelConfigTestProps> = (props) => {
  const [availableModels, setAvailableModels] = useState<ModelOption[]>([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelListError, setModelListError] = useState('')

  const credentialState = `${props.apiKeyConfigured ? 'configured' : 'unconfigured'}:${props.clearApiKey ? 'clear' : 'keep'}`
  const modelOptionsKey = resetModelOptionsKey(props.type, props.provider, props.baseUrl, props.apiKey, credentialState)
  const currentModelOptionsKey = useRef(modelOptionsKey)
  const discoveryGeneration = useRef(0)

  currentModelOptionsKey.current = modelOptionsKey

  useEffect(() => {
    discoveryGeneration.current += 1
    setAvailableModels([])
    setModelListError('')
    setLoadingModels(false)
  }, [modelOptionsKey])

  const handleDiscovery = async () => {
    if (!props.baseUrl.trim() || loadingModels) {
      return
    }

    const requestKey = modelOptionsKey
    const requestGeneration = discoveryGeneration.current + 1
    discoveryGeneration.current = requestGeneration
    const isCurrentRequest = () => isCurrentModelDiscoveryRequest(
      requestKey,
      requestGeneration,
      currentModelOptionsKey.current,
      discoveryGeneration.current,
    )

    setLoadingModels(true)
    setModelListError('')

    try {
      const nextResult = await fetchAvailableModels({
        provider: props.provider,
        baseUrl: props.baseUrl,
        apiKey: props.apiKey,
        apiKeyConfigured: props.apiKeyConfigured,
        clearApiKey: props.clearApiKey,
      }, props.type)

      if (!isCurrentRequest()) {
        return
      }

      if (nextResult.success) {
        setAvailableModels(nextResult.models ?? [])
      } else {
        setModelListError(nextResult.error_message || '获取模型失败')
      }
    } catch (error) {
      if (isCurrentRequest()) {
        setModelListError(error instanceof Error ? error.message : '获取模型请求失败')
      }
    } finally {
      if (isCurrentRequest()) {
        setLoadingModels(false)
      }
    }
  }

  return (
    <div className="model-config-test">
      <div className="model-discovery-controls">
        <button
          type="button"
          className="test-connection-btn"
          onClick={() => void handleDiscovery()}
          disabled={!props.baseUrl.trim() || loadingModels}
        >
          {loadingModels ? '获取中...' : '获取模型'}
        </button>
      </div>

      {availableModels.length > 0 ? (
        <select
          aria-label="可用模型"
          className="model-discovery-select"
          value={props.modelName}
          onChange={(event) => props.onModelChange(event.target.value)}
        >
          <option value="">请选择候选模型</option>
          {availableModels.map((option) => (
            <option
              key={option.id}
              value={option.name}
              disabled={option.capability_status === 'unsupported'}
            >
              {modelOptionLabel(option)}
            </option>
          ))}
        </select>
      ) : null}

      {modelListError ? (
        <div className="model-discovery-error" role="alert">
          {modelListError}
        </div>
      ) : null}
    </div>
  )
}
