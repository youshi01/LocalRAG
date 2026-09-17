import React, { useEffect, useRef, useState } from 'react'
import { fetchAvailableModels, probeModel } from '../../services/api'
import type { ChatConfig, EmbeddingConfig } from '../../App'
import type { ModelOption, ModelProbeResponse } from '../../services/api'
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
      temperature: number
      onModelChange: (value: string) => void
    }
  | {
      type: 'embedding'
      provider: EmbeddingConfig['provider']
      baseUrl: string
      modelName: string
      apiKey: string
      temperature?: never
      onModelChange: (value: string) => void
    }

const defaultSuccessMessage: Record<ModelConfigTestProps['type'], string> = {
  chat: '聊天模型连接正常',
  embedding: 'Embedding 模型连接正常',
}

export const ModelConfigTest: React.FC<ModelConfigTestProps> = (props) => {
  const [availableModels, setAvailableModels] = useState<ModelOption[]>([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelListError, setModelListError] = useState('')
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<ModelProbeResponse | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  const hasRequiredConfig = Boolean(props.baseUrl.trim() && props.modelName.trim())
  const modelOptionsKey = resetModelOptionsKey(props.type, props.provider, props.baseUrl)
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
      }, props.type)

      if (!isCurrentRequest()) {
        return
      }

      if (nextResult.success) {
        setAvailableModels(nextResult.models)
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

  const handleTest = async () => {
    if (!hasRequiredConfig || testing) {
      return
    }

    setTesting(true)
    setResult(null)
    setErrorMessage('')

    try {
      const nextResult =
        props.type === 'chat'
          ? await probeModel({
              provider: props.provider,
              baseUrl: props.baseUrl,
              model: props.modelName,
              apiKey: props.apiKey,
              temperature: props.temperature,
              knowledgeTemperature: 0.1,
              contextMessageLimit: 1,
            }, props.type)
          : await probeModel({
              provider: props.provider,
              baseUrl: props.baseUrl,
              model: props.modelName,
              apiKey: props.apiKey,
            }, props.type)

      setResult(nextResult)
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '模型测试请求失败')
    } finally {
      setTesting(false)
    }
  }

  const message =
    result?.success
      ? defaultSuccessMessage[props.type]
      : result?.error_message ||
        errorMessage ||
        (result ? '模型探测失败' : hasRequiredConfig ? '' : '请先填写 Base URL 和 Model')

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
        <button
          type="button"
          className="test-connection-btn"
          onClick={() => void handleTest()}
          disabled={!hasRequiredConfig || testing}
        >
          {testing ? '探测中...' : '探测模型'}
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
            <option key={option.id} value={option.name}>
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

      {message ? (
        <div
          className={`test-result ${
            result?.success ? 'test-success' : 'test-error'
          }`}
          role="status"
          aria-live="polite"
        >
          <span className="test-icon">{result?.success ? 'OK' : '!'}</span>
          <div className="test-details">
            <span className="test-message">{message}</span>
            {result?.model_info ? (
              <span className="test-info">{result.model_info}</span>
            ) : null}
            {typeof result?.latency_ms === 'number' ? (
              <span className="test-latency">{result.latency_ms} ms</span>
            ) : null}
            {typeof result?.vector_size === 'number' ? (
              <span className="test-info">模型输出维度：{result.vector_size}</span>
            ) : null}
            {typeof result?.expected_vector_size === 'number' ? (
              <span className="test-info">
                Qdrant 配置维度：{result.expected_vector_size}
              </span>
            ) : null}
            {!result?.success && result?.error_message ? (
              <span className="test-error-msg">{result.error_message}</span>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  )
}
