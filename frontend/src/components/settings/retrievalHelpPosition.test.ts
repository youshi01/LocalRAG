import { describe, expect, it } from 'vitest'
import { getRetrievalHelpPosition } from './retrievalHelpPosition'

describe('retrieval help popover positioning', () => {
  const viewport = { width: 552, height: 987 }

  it('keeps a right-edge popover fully inside the viewport', () => {
    expect(getRetrievalHelpPosition(
      { top: 90, right: 468, bottom: 112, left: 446 },
      { width: 360, height: 300 },
      viewport,
    )).toEqual({ top: 120, left: 108 })
  })

  it('opens above the trigger when there is not enough room below', () => {
    expect(getRetrievalHelpPosition(
      { top: 900, right: 520, bottom: 922, left: 498 },
      { width: 320, height: 260 },
      viewport,
    )).toEqual({ top: 632, left: 200 })
  })

  it('clamps a wide popover to the viewport margins', () => {
    expect(getRetrievalHelpPosition(
      { top: 120, right: 180, bottom: 142, left: 158 },
      { width: 600, height: 200 },
      viewport,
    )).toEqual({ top: 150, left: 12 })
  })
})
