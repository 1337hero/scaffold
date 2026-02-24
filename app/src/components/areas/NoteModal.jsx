import { useState, useEffect } from "preact/hooks"
import { nullable } from "@/utils/nullable.js"

const NoteModal = ({ note, domainId, initialEditMode = false, onClose, onSave, onDelete }) => {
  const [editing, setEditing] = useState(note === null || initialEditMode)
  const [title, setTitle] = useState(note?.Title ?? "")
  const [content, setContent] = useState(nullable(note?.Content) ?? "")

  useEffect(() => {
    document.body.style.overflow = "hidden"
    return () => { document.body.style.overflow = "" }
  }, [])

  useEffect(() => {
    const handleKey = (e) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onClose])

  function handleSave() {
    if (note) {
      onSave(note.ID, { title: title.trim(), content })
    } else {
      onSave({ title: title.trim(), content })
    }
    onClose()
  }

  function handleDelete() {
    onDelete(note.ID)
    onClose()
  }

  function handleCancel() {
    if (note === null) {
      onClose()
      return
    }
    setTitle(note.Title)
    setContent(nullable(note.Content) ?? "")
    setEditing(false)
  }

  return (
    <div
      class="fixed inset-0 bg-black/50 z-50 flex items-center justify-center"
      onClick={onClose}
    >
      <div
        class="w-[80vw] max-h-[80vh] rounded-3xl bg-[var(--color-card-bg)] border border-app-border card-shadow flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div class="p-6 pb-0 flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            {editing ? (
              <input
                type="text"
                value={title}
                onInput={(e) => setTitle(e.currentTarget.value)}
                class="w-full text-xl font-bold bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all"
                placeholder="Title"
                autoFocus
              />
            ) : (
              <h2 class="text-xl font-bold font-serif">{note?.Title}</h2>
            )}
          </div>
          <div class="flex items-center gap-3 shrink-0">
            {!editing && note && (
              <button
                type="button"
                onClick={() => setEditing(true)}
                class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer"
              >
                Edit
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              class="text-app-muted hover:text-app-ink transition-colors cursor-pointer"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 6 6 18" />
                <path d="m6 6 12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Body */}
        <div class="p-6 flex-1 overflow-y-auto">
          {editing ? (
            <textarea
              value={content}
              onInput={(e) => setContent(e.currentTarget.value)}
              class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm text-app-muted outline-none focus:border-app-ink/30 transition-all resize-y min-h-[200px]"
              placeholder="Write something..."
            />
          ) : (
            <div class="text-sm text-app-muted whitespace-pre-wrap leading-relaxed">
              {nullable(note?.Content) || ""}
            </div>
          )}
        </div>

        {/* Footer */}
        <div class="p-6 pt-0 flex items-center justify-between">
          <div>
            {onDelete && note && (
              <button
                type="button"
                onClick={handleDelete}
                class="text-[10px] mono uppercase font-bold text-red-400 hover:text-red-600 transition-colors cursor-pointer"
              >
                Delete
              </button>
            )}
          </div>
          <div class="flex items-center gap-2">
            {editing && (
              <>
                <button
                  type="button"
                  onClick={handleCancel}
                  class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={!title.trim()}
                  class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                >
                  Save
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default NoteModal
