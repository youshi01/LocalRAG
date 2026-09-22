import { describe, expect, it } from 'vitest'
import {
  isCurrentModelDiscoveryRequest,
  modelOptionLabel,
  modelProbeKey,
  resetModelOptionsKey,
} from './modelOptions'

describe('model option UI helpers', () => {
  it('changes the reset key when the normalized endpoint changes', () => {
    const original = resetModelOptionsKey('chat', ' Ollama ', 'http://localhost:11434/v1/')

    expect(resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434/v1')).toBe(original)
    expect(resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434/v2')).not.toBe(original)
  })

  it('changes the reset key when the API key changes without exposing the key', () => {
    const first = resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434', 'first-secret')
    const second = resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434', 'second-secret')

    expect(first).not.toBe(second)
    expect(first).not.toContain('first-secret')
    expect(second).not.toContain('second-secret')
  })

  it('changes the reset key when stored-key intent changes', () => {
    const stored = resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434', '', 'configured:keep')
    const cleared = resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434', '', 'configured:clear')

    expect(stored).not.toBe(cleared)
  })

  it('renders an owner only when it is present', () => {
    expect(modelOptionLabel({ id: 'qwen3', name: 'qwen3', type: 'chat', owned_by: 'Alibaba' })).toBe('qwen3 · Alibaba')
    expect(modelOptionLabel({ id: 'local', name: 'local', type: 'chat', owned_by: '' })).toBe('local')
  })

  it('labels known, unsupported, and unknown model capabilities', () => {
    expect(modelOptionLabel({
      id: 'embed', name: 'embed', type: 'embedding', owned_by: '', capability_status: 'supported',
    })).toContain('能力匹配')
    expect(modelOptionLabel({
      id: 'chat', name: 'chat', type: 'embedding', owned_by: '', capability_status: 'unsupported',
    })).toContain('不支持当前类型')
    expect(modelOptionLabel({
      id: 'remote', name: 'remote', type: 'embedding', owned_by: '', capability_status: 'unknown',
    })).toContain('能力待确认')
  })

  it('rejects a discovery result after its endpoint or request generation is superseded', () => {
    const oldKey = resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434')
    const nextKey = resetModelOptionsKey('chat', 'openai-compatible', 'http://localhost:11434/v1')

    expect(isCurrentModelDiscoveryRequest(oldKey, 4, nextKey, 5)).toBe(false)
    expect(isCurrentModelDiscoveryRequest(nextKey, 5, nextKey, 5)).toBe(true)
  })

  it('changes the probe key when the selected model or temperature changes', () => {
    const original = modelProbeKey('chat', 'ollama', 'http://localhost:11434', 'model-a', 'secret', 0.2)

    expect(modelProbeKey('chat', 'ollama', 'http://localhost:11434', 'model-b', 'secret', 0.2)).not.toBe(original)
    expect(modelProbeKey('chat', 'ollama', 'http://localhost:11434', 'model-a', 'secret', 0.5)).not.toBe(original)
  })
})
