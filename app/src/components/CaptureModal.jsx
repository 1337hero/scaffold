import { useEffect, useRef, useState } from "preact/hooks"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { createCapture } from "@/api/queries.js"

const CaptureModal = ({ open, onClose }) => {
  const dialogRef = useRef(null)
  const inputRef = useRef(null)
  const [text, setText] = useState("")
  const [error, setError] = useState("")
  const [showToast, setShowToast] = useState(false)
  const queryClient = useQueryClient()

  const captureMutation = useMutation({
    mutationFn: (captureText) => createCapture(captureText),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["inbox"] })
      queryClient.invalidateQueries({ queryKey: ["inbox-count"] })
      setText("")
      setError("")
      onClose()
      setShowToast(true)
      setTimeout(() => setShowToast(false), 2000)
    },
    onError: (err) => {
      setError(err?.message ?? "Failed to capture")
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
      setText("")
      setError("")
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
    setError("")
    captureMutation.mutate(trimmed)
  }

  return (
    <>
      <dialog
        ref={dialogRef}
        class="m-0 p-0 w-screen h-screen max-w-none max-h-none bg-transparent border-none backdrop:bg-black/20 backdrop:backdrop-blur-sm"
        onCancel={onCancel}
      >
        <div
          class="w-full h-full flex items-center justify-center p-4"
          onClick={(e) => { if (e.target === e.currentTarget && !captureMutation.isPending) onClose() }}
        >
          <div class="modal-card relative w-full max-w-xl bg-white rounded-3xl shadow-2xl overflow-hidden p-6">
            <form onSubmit={onSubmit} class="space-y-4">
              <div class="flex items-center gap-3 text-app-muted mb-2">
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                  <path d="M5 12h14" /><path d="M12 5v14" />
                </svg>
                <span class="text-xs font-bold font-mono uppercase tracking-widest">Quick Capture</span>
              </div>

              <input
                ref={inputRef}
                type="text"
                value={text}
                onInput={(e) => setText(e.currentTarget.value)}
                class="w-full text-xl font-medium text-app-ink bg-transparent focus:outline-none placeholder:text-app-ink/30"
                placeholder="What's on your mind?"
                disabled={captureMutation.isPending}
              />

              {error && (
                <div class="rounded-xl border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-600">
                  {error}
                </div>
              )}

              <div class="flex justify-between items-center pt-4 border-t border-app-border">
                <span class="text-[10px] font-mono text-app-muted/50">Press Enter to save</span>
                <button
                  type="submit"
                  disabled={captureMutation.isPending || !text.trim()}
                  class="px-6 py-2 bg-app-ink text-white rounded-xl font-bold text-sm hover:bg-black/80 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {captureMutation.isPending ? "Capturing\u2026" : "Capture"}
                </button>
              </div>
            </form>
          </div>
        </div>
      </dialog>

      <div
        class={`fixed bottom-6 left-1/2 -translate-x-1/2 bg-app-ink text-white rounded-xl px-5 py-2.5 text-sm font-medium shadow-lg z-50 transition-opacity duration-300 pointer-events-none ${showToast ? "opacity-100" : "opacity-0"}`}
      >
        Captured. It&rsquo;s in your inbox.
      </div>
    </>
  )
}

export default CaptureModal
