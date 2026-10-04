export interface RetrievalHelpInteractionState {
  triggerHovered: boolean
  popoverHovered: boolean
  triggerFocused: boolean
  popoverFocused: boolean
}

export const shouldKeepRetrievalHelpOpen = (state: RetrievalHelpInteractionState) => (
  state.triggerHovered ||
  state.popoverHovered ||
  state.triggerFocused ||
  state.popoverFocused
)
