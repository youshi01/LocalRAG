import { describe, expect, it } from 'vitest'
import { modelOptionLabel, resetModelOptionsKey } from './modelOptions'

describe('model option UI helpers', () => {
  it('changes the reset key when the normalized endpoint changes', () => {
    const original = resetModelOptionsKey('chat', ' Ollama ', 'http://localhost:11434/v1/')

    expect(resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434/v1')).toBe(original)
    expect(resetModelOptionsKey('chat', 'ollama', 'http://localhost:11434/v2')).not.toBe(original)
  })

  it('renders an owner only when it is present', () => {
    expect(modelOptionLabel({ id: 'qwen3', name: 'qwen3', type: 'chat', owned_by: 'Alibaba' })).toBe('qwen3 · Alibaba')
    expect(modelOptionLabel({ id: 'local', name: 'local', type: 'chat', owned_by: '' })).toBe('local')
  })
})