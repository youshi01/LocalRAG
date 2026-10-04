import React, { useRef } from 'react'
import { useModalFocusTrap } from '../../hooks/useModalFocusTrap'

interface ConfirmDialogProps {
  open: boolean
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  confirmDisabled?: boolean
  cancelDisabled?: boolean
  onConfirm: () => void
  onCancel: () => void
}

const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  open,
  title = '确认',
  message,
  confirmText = '确认',
  cancelText = '取消',
  confirmDisabled = false,
  cancelDisabled = false,
  onConfirm,
  onCancel,
}) => {
  const backdropRef = useRef<HTMLDivElement | null>(null)
  const cancelButtonRef = useRef<HTMLButtonElement | null>(null)

  useModalFocusTrap(backdropRef, {
    enabled: open,
    initialFocusRef: cancelButtonRef,
    onClose: cancelDisabled ? undefined : onCancel,
  })

  if (!open) return null

  return (
    <div className="confirm-backdrop" onClick={cancelDisabled ? undefined : onCancel} ref={backdropRef}>
      <div
        aria-describedby="confirm-dialog-message"
        aria-labelledby="confirm-dialog-title"
        aria-modal="true"
        className="confirm-dialog"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
      >
        <div className="confirm-header" id="confirm-dialog-title">{title}</div>
        <div className="confirm-body" id="confirm-dialog-message">{message}</div>
        <div className="confirm-footer">
          <button type="button" className="confirm-btn confirm-btn--cancel" onClick={onCancel} ref={cancelButtonRef} disabled={cancelDisabled}>
            {cancelText}
          </button>
          <button type="button" className="confirm-btn confirm-btn--confirm" onClick={onConfirm} disabled={confirmDisabled}>
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  )
}

export default ConfirmDialog
