import { describe, expect, it } from 'vitest'
import type { HealthSummaryResponse } from '../../services/api'
import { summarizeHealthWarning } from './settingsHealth'

const healthy: HealthSummaryResponse = {
  qdrant: { status: 'ok' },
  chat_model: { status: 'ok' },
  embedding_model: { status: 'ok' },
  storage: { status: 'ok' },
  auth: { status: 'ok' },
}

describe('settings health feedback', () => {
  it('returns no warning when all saved dependencies are healthy', () => {
    expect(summarizeHealthWarning(healthy)).toBe('')
  })

  it('explains failed model health after a successful save', () => {
    expect(summarizeHealthWarning({
      ...healthy,
      embedding_model: { status: 'error', error_message: 'Embedding endpoint unavailable' },
    })).toContain('Embedding')
  })
})
