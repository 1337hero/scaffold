import { useState } from "preact/hooks"
import { nullable } from "@/utils/nullable.js"

const NoteItem = ({ note, onSave, onDelete }) => {
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState(note.Title)
  const [content, setContent] = useState(nullable(note.Content) || "")

  const preview = nullable(note.Content)

  function handleSave() {
    onSave(note.ID, { title, content })
    setEditing(false)
  }

  function handleCancel() {
    setTitle(note.Title)
    setContent(nullable(note.Content) || "")
    setEditing(false)
  }

  if (editing) {
    return (
      <div class="p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow space-y-3">
        <input
          type="text"
          value={title}
          onInput={(e) => setTitle(e.currentTarget.value)}
          class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm font-bold outline-none focus:border-app-ink/30 transition-all"
          placeholder="Title"
        />
        <input
          type="text"
          value={content}
          onInput={(e) => setContent(e.currentTarget.value)}
          class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm text-app-muted outline-none focus:border-app-ink/30 transition-all"
          placeholder="Content..."
        />
        <div class="flex items-center gap-2 justify-end">
          <button
            type="button"
            onClick={handleCancel}
            class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={!title.trim()}
            class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40"
          >
            Save
          </button>
        </div>
      </div>
    )
  }

  return (
    <div class="p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow space-y-2 group">
      <div class="flex items-start justify-between gap-3">
        <h4 class="font-bold">{note.Title}</h4>
        <div class="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
          <button
            type="button"
            onClick={() => setEditing(true)}
            class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer"
          >
            Edit
          </button>
          <button
            type="button"
            onClick={() => onDelete(note.ID)}
            class="text-[10px] mono uppercase font-bold text-app-muted hover:text-red-500 transition-colors cursor-pointer"
          >
            Del
          </button>
        </div>
      </div>
      {preview && (
        <p class="text-sm text-app-muted line-clamp-2">{preview}</p>
      )}
    </div>
  )
}

export default NoteItem
