import { describe, expect, it } from 'vitest'
import { shouldKeepRetrievalHelpOpen } from './retrievalHelpHover'

describe('retrieval help hover state', () => {
  it('keeps the explanation visible while the pointer or focus is inside it', () => {
    expect(shouldKeepRetrievalHelpOpen({
      triggerHovered: true,
      popoverHovered: false,
      triggerFocused: false,
      popoverFocused: false,
    })).toBe(true)
    expect(shouldKeepRetrievalHelpOpen({
      triggerHovered: false,
      popoverHovered: true,
      triggerFocused: false,
      popoverFocused: false,
    })).toBe(true)
    expect(shouldKeepRetrievalHelpOpen({
      triggerHovered: false,
      popoverHovered: false,
      triggerFocused: false,
      popoverFocused: true,
    })).toBe(true)
  })

  it('allows the explanation to disappear after leaving both controls', () => {
    expect(shouldKeepRetrievalHelpOpen({
      triggerHovered: false,
      popoverHovered: false,
      triggerFocused: false,
      popoverFocused: false,
    })).toBe(false)
  })
})
