import { useEffect, useId, useRef, type ReactNode } from 'react'
import { focusRing } from '../../lib/cn'

type ModalProps = {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer?: ReactNode
}

// Usa <dialog> nativo: aporta trampa de foco, Escape y aria-modal.
export function Modal({ open, onClose, title, children, footer }: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const titleId = useId()

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    if (!open && dialog.open) dialog.close()
  }, [open])

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={titleId}
      onClose={onClose}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
      className="m-auto w-[min(90vw,36rem)] rounded-2xl p-0 text-neutral-900 shadow-xl backdrop:bg-neutral-900/60"
    >
      <div className="flex flex-col gap-4 p-6">
        <div className="flex items-start justify-between gap-4">
          <h2 id={titleId} className="text-2xl font-bold">
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Cerrar"
            className={`-m-2 flex size-12 shrink-0 items-center justify-center rounded-xl text-3xl leading-none text-neutral-700 hover:bg-neutral-100 ${focusRing}`}
          >
            <span aria-hidden="true">×</span>
          </button>
        </div>
        <div className="text-lg text-neutral-800">{children}</div>
        {footer && <div className="flex flex-wrap justify-end gap-3">{footer}</div>}
      </div>
    </dialog>
  )
}
