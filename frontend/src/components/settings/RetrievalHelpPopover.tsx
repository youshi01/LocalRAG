import React, { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { RetrievalHelpSection } from './retrievalHelp'
import { getRetrievalHelpPosition, type RetrievalHelpPosition } from './retrievalHelpPosition'
import {
  shouldKeepRetrievalHelpOpen,
  type RetrievalHelpInteractionState,
} from './retrievalHelpHover'

interface RetrievalHelpProps {
  section: RetrievalHelpSection
}

const HOVER_CLOSE_DELAY_MS = 140

type InteractionTarget = keyof RetrievalHelpInteractionState

const initialInteractionState: RetrievalHelpInteractionState = {
  triggerHovered: false,
  popoverHovered: false,
  triggerFocused: false,
  popoverFocused: false,
}

const RetrievalHelp: React.FC<RetrievalHelpProps> = ({ section }) => {
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState<RetrievalHelpPosition | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const closeTimerRef = useRef<number | null>(null)
  const interactionStateRef = useRef<RetrievalHelpInteractionState>({ ...initialInteractionState })
  const id = useId().replace(/:/g, '')
  const popoverId = `retrieval-help-${id}`

  const clearCloseTimer = useCallback(() => {
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }, [])

  const scheduleClose = useCallback(() => {
    clearCloseTimer()
    closeTimerRef.current = window.setTimeout(() => {
      closeTimerRef.current = null
      if (!shouldKeepRetrievalHelpOpen(interactionStateRef.current)) {
        setOpen(false)
      }
    }, HOVER_CLOSE_DELAY_MS)
  }, [clearCloseTimer])

  const updateInteraction = useCallback((target: InteractionTarget, active: boolean) => {
    interactionStateRef.current[target] = active
    if (active) {
      clearCloseTimer()
      setOpen(true)
      return
    }
    scheduleClose()
  }, [clearCloseTimer, scheduleClose])

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current
    const popover = popoverRef.current
    if (!trigger || !popover) return

    const anchor = trigger.getBoundingClientRect()
    const size = popover.getBoundingClientRect()
    setPosition(getRetrievalHelpPosition(
      {
        top: anchor.top,
        right: anchor.right,
        bottom: anchor.bottom,
        left: anchor.left,
      },
      { width: size.width, height: size.height },
      { width: window.innerWidth, height: window.innerHeight },
    ))
  }, [])

  useLayoutEffect(() => {
    if (!open) {
      setPosition(null)
      return undefined
    }

    updatePosition()
    const handleViewportChange = () => updatePosition()
    window.addEventListener('resize', handleViewportChange)
    window.addEventListener('scroll', handleViewportChange, true)
    return () => {
      window.removeEventListener('resize', handleViewportChange)
      window.removeEventListener('scroll', handleViewportChange, true)
    }
  }, [open, updatePosition])

  useEffect(() => () => clearCloseTimer(), [clearCloseTimer])

  const popover = open && typeof document !== 'undefined'
    ? createPortal(
      <div
        ref={popoverRef}
        id={popoverId}
        className="settings-help-popover"
        role="dialog"
        aria-label={section.title}
        onMouseEnter={() => updateInteraction('popoverHovered', true)}
        onMouseLeave={() => updateInteraction('popoverHovered', false)}
        onFocus={() => updateInteraction('popoverFocused', true)}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
            updateInteraction('popoverFocused', false)
          }
        }}
        style={position ? { top: position.top, left: position.left } : undefined}
      >
        <div className="settings-help-popover-head">
          <strong>{section.title}</strong>
        </div>
        <p>{section.summary}</p>
        <ul>
          {section.items.map((item) => (
            <li key={item.label}>
              <strong>{item.label}</strong>
              <span>{item.description}</span>
            </li>
          ))}
        </ul>
      </div>,
      document.body,
    )
    : null

  return (
    <div className="settings-help">
      <button
        ref={triggerRef}
        type="button"
        className="settings-help-trigger"
        aria-label={`查看${section.title}`}
        aria-expanded={open}
        aria-controls={popoverId}
        aria-haspopup="dialog"
        onMouseEnter={() => updateInteraction('triggerHovered', true)}
        onMouseLeave={() => updateInteraction('triggerHovered', false)}
        onFocus={() => updateInteraction('triggerFocused', true)}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
            updateInteraction('triggerFocused', false)
          }
        }}
      >
        ？
      </button>
      {popover}
    </div>
  )
}

export default RetrievalHelp
