import { useEffect, useRef, useState } from 'preact/hooks'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createCapture } from '@/api/queries.js'

export function CaptureModal({ open, onClose }) {
  const inputRef = useRef(null)
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const queryClient = useQueryClient()

  const captureMutation = useMutation({
    mutationFn: (captureText) => createCapture(captureText),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inbox'] })
      queryClient.invalidateQueries({ queryKey: ['memories'] })
      setText('')
      setError('')
      onClose()
    },
    onError: (err) => {
      setError(err?.message ?? 'Failed to capture')
    },
  })

  useEffect(() => {
    if (!open) {
      setText('')
      setError('')
      captureMutation.reset()
      return
    }

    if (inputRef.current) {
      inputRef.current.focus()
    }
  }, [open])

  function onOverlayClick(e) {
    if (captureMutation.isPending) return
    if (e.target === e.currentTarget) onClose()
  }

  function onKeyDown(e) {
    if (e.key === 'Escape' && !captureMutation.isPending) {
      e.preventDefault()
      onClose()
    }
  }

  function onSubmit(e) {
    e.preventDefault()
    const trimmed = text.trim()
    if (!trimmed || captureMutation.isPending) return

    setError('')
    captureMutation.mutate(trimmed)
  }

  if (!open) return null

  return (
    <div
      class="fixed inset-0 bg-black/60 z-[200] flex items-center justify-center backdrop-blur-[4px]"
      onClick={onOverlayClick}
      role="dialog"
      aria-modal="true"
      aria-label="Capture modal"
      onKeyDown={onKeyDown}
    >
      <div class="bg-surface border border-border rounded-[16px] p-6 w-[90%] max-w-[560px] shadow-[0_18px_50px_rgba(0,0,0,0.45)]">
        <form onSubmit={onSubmit}>
          <input
            ref={inputRef}
            type="text"
            value={text}
            onInput={(e) => setText(e.currentTarget.value)}
            class="w-full py-4 px-[18px] bg-surface-2 border border-border rounded-[10px] text-text font-sans text-base focus:border-amber"
            placeholder="Thought, link, idea, task..."
            disabled={captureMutation.isPending}
          />

          {error && (
            <div class="mt-3 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-[0.78rem] text-red-300">
              {error}
            </div>
          )}

          <div class="mt-4 flex items-center justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              disabled={captureMutation.isPending}
              class="text-[0.74rem] font-medium py-1.5 px-3 rounded-md border border-border text-text-dim hover:bg-surface-3 hover:text-text font-sans cursor-pointer transition-all disabled:opacity-60 disabled:cursor-not-allowed"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={captureMutation.isPending || !text.trim()}
              class="text-[0.74rem] font-semibold py-1.5 px-3 rounded-md border border-amber-border text-amber bg-amber-dim hover:bg-[rgba(245,158,11,0.18)] font-sans cursor-pointer transition-all disabled:opacity-60 disabled:cursor-not-allowed"
            >
              {captureMutation.isPending ? 'Capturing…' : 'Capture'}
            </button>
          </div>
        </form>

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
