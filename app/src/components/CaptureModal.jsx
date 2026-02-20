import { useRef, useEffect } from 'preact/hooks'

export function CaptureModal({ open, onClose }) {
  const inputRef = useRef(null)

  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus()
    }
  }, [open])

  function onOverlayClick(e) {
    if (e.target === e.currentTarget) onClose()
  }

  function onKeyDown(e) {
    if (e.key === 'Enter' && inputRef.current?.value.trim()) {
      inputRef.current.value = ''
      onClose()
    }
  }

  if (!open) return null

  return (
    <div
      class="fixed inset-0 bg-black/60 z-[200] flex items-center justify-center backdrop-blur-[4px]"
      onClick={onOverlayClick}
      role="dialog"
      aria-modal="true"
      aria-label="Capture modal"
    >
      <div class="bg-surface border border-border rounded-[16px] p-6 w-[90%] max-w-[560px] shadow-[0_18px_50px_rgba(0,0,0,0.45)]">
        <input
          ref={inputRef}
          type="text"
          class="w-full py-4 px-[18px] bg-surface-2 border border-border rounded-[10px] text-text font-sans text-base focus:border-amber"
          placeholder="Thought, link, idea, task..."
          onKeyDown={onKeyDown}
        />
        <div class="text-[0.75rem] text-text-muted flex justify-between mt-3">
          <span>AI will classify automatically</span>
          <span>
            <kbd class="font-mono bg-surface-3 px-1.5 py-0.5 rounded-sm text-[0.7rem]">Esc</kbd>
            {' to close \u00b7 '}
            <kbd class="font-mono bg-surface-3 px-1.5 py-0.5 rounded-sm text-[0.7rem]">Enter</kbd>
            {' to capture'}
          </span>
        </div>
      </div>
    </div>
  )
}
