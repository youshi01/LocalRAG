export interface RetrievalHelpAnchorRect {
  top: number
  right: number
  bottom: number
  left: number
}

export interface RetrievalHelpPopoverSize {
  width: number
  height: number
}

export interface RetrievalHelpViewport {
  width: number
  height: number
}

export interface RetrievalHelpPosition {
  top: number
  left: number
}

export const getRetrievalHelpPosition = (
  anchor: RetrievalHelpAnchorRect,
  popover: RetrievalHelpPopoverSize,
  viewport: RetrievalHelpViewport,
  gap = 8,
  margin = 12,
): RetrievalHelpPosition => {
  const maxLeft = Math.max(margin, viewport.width - popover.width - margin)
  const preferredLeft = anchor.right - popover.width
  const left = Math.min(Math.max(preferredLeft, margin), maxLeft)

  const preferredBelowTop = anchor.bottom + gap
  const preferredAboveTop = anchor.top - popover.height - gap
  const maxTop = Math.max(margin, viewport.height - popover.height - margin)
  const hasRoomBelow = preferredBelowTop + popover.height <= viewport.height - margin
  const hasRoomAbove = preferredAboveTop >= margin
  const top = hasRoomBelow
    ? preferredBelowTop
    : hasRoomAbove
      ? preferredAboveTop
      : Math.min(Math.max(preferredBelowTop, margin), maxTop)

  return { top, left }
}
