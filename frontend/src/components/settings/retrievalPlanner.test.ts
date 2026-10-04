import { describe, expect, it } from 'vitest'
import { createDefaultAppConfig, normalizeAppConfig } from '../../app/appConfig'
import type { AppConfig } from '../../App'

const fallback = createDefaultAppConfig()

describe('model-assisted retrieval config', () => {
  it('enables bounded model planning by default', () => {
    expect(fallback.retrieval.enableModelRetrievalPlanner).toBe(true)
    expect(fallback.retrieval.modelRetrievalMaxRounds).toBe(2)
  })

  it('normalizes legacy and invalid round values', () => {
    const normalized = normalizeAppConfig({
      retrieval: {
        ...fallback.retrieval,
        modelRetrievalMaxRounds: 99,
      },
    }, fallback)
    expect(normalized.retrieval.modelRetrievalMaxRounds).toBe(2)

    const legacy = normalizeAppConfig({ retrieval: {} } as Partial<AppConfig>, fallback)
    expect(legacy.retrieval.modelRetrievalMaxRounds).toBe(2)
  })

  it('preserves an explicitly disabled planner', () => {
    const normalized = normalizeAppConfig({
      retrieval: {
        ...fallback.retrieval,
        enableModelRetrievalPlanner: false,
        modelRetrievalMaxRounds: 1,
      },
    }, fallback)
    expect(normalized.retrieval.enableModelRetrievalPlanner).toBe(false)
    expect(normalized.retrieval.modelRetrievalMaxRounds).toBe(1)
  })
})
