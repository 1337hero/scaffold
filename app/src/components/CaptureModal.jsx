import { useEffect, useRef, useState } from 'preact/hooks'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createCapture } from '@/api/queries.js'

export function CaptureModal({ open, onClose }) {
  const dialogRef = useRef(null)
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
    const dialog = dialogRef.current
    if (!dialog) return
    if (open) {
      if (!dialog.open) dialog.showModal()
      inputRef.current?.focus()
    } else {
      if (dialog.open) dialog.close()
      setText('')
      setError('')
      captureMutation.reset()
    }
  }, [open])

  function onCancel(e) {
    e.preventDefault()
    if (!captureMutation.isPending) onClose()
  }

  function onSubmit(e) {
    e.preventDefault()
    const trimmed = text.trim()
    if (!trimmed || captureMutation.isPending) return
    setError('')
    captureMutation.mutate(trimmed)
  }

  return (
    <dialog
      ref={dialogRef}
      class="m-0 p-0 w-screen h-screen max-w-none max-h-none bg-transparent border-none backdrop:bg-black/60 backdrop:backdrop-blur-[4px]"
      onCancel={onCancel}
    >
      <div
        class="w-full h-full flex items-center justify-center"
        onClick={(e) => { if (e.target === e.currentTarget && !captureMutation.isPending) onClose() }}
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
              class="btn-amber text-[0.74rem] py-1.5 px-3 rounded-md"
            >
              {captureMutation.isPending ? 'Capturing\u2026' : 'Capture'}
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
    </dialog>
  )
}
