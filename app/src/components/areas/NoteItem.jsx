import { useState } from "preact/hooks"
import { nullable } from "@/utils/nullable.js"
import NoteModal from "./NoteModal.jsx"

const NoteItem = ({ note, onSave, onDelete }) => {
  const [modalOpen, setModalOpen] = useState(false)
  const [modalEditMode, setModalEditMode] = useState(false)

  const preview = nullable(note.Content)

  return (
    <>
      <div
        class="p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow space-y-2 group cursor-pointer"
        onClick={() => { setModalEditMode(false); setModalOpen(true) }}
      >
        <div class="flex items-start justify-between gap-3">
          <h4 class="font-bold">{note.Title}</h4>
          <div class="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); setModalEditMode(true); setModalOpen(true) }}
              class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer"
            >
              Edit
            </button>
          </div>
        </div>
        {preview && (
          <p class="text-sm text-app-muted line-clamp-3">{preview}</p>
        )}
      </div>
      {modalOpen && (
        <NoteModal
          note={note}
          initialEditMode={modalEditMode}
          onClose={() => setModalOpen(false)}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </>
  )
}

export default NoteItem
