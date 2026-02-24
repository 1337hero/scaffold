import { useState } from "preact/hooks"
import TaskModal from "./TaskModal.jsx"

const CircleIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
  </svg>
)

const CheckCircleIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <path d="m9 12 2 2 4-4" />
  </svg>
)

const TaskItem = ({ task, onComplete, onSave, onDelete, domains, goals }) => {
  const [modalOpen, setModalOpen] = useState(false)
  const [modalEditMode, setModalEditMode] = useState(false)
  const done = task.Status === "done"

  return (
    <>
      <div
        class="p-4 flex items-center gap-4 group cursor-pointer"
        onClick={() => { setModalEditMode(false); setModalOpen(true) }}
      >
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); !done && onComplete?.() }}
          class={`transition-colors shrink-0 ${done ? "text-emerald-500 cursor-default" : "text-app-muted hover:text-app-ink cursor-pointer"}`}
        >
          {done ? <CheckCircleIcon /> : <CircleIcon />}
        </button>
        <span class={`text-sm font-medium flex-1 ${done ? "line-through opacity-40" : ""}`}>
          {task.Title}
        </span>
        <div class="flex gap-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
          <button type="button"
            onClick={(e) => { e.stopPropagation(); setModalEditMode(true); setModalOpen(true) }}
            class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer">
            Edit
          </button>
        </div>
      </div>
      {modalOpen && (
        <TaskModal
          task={task}
          domains={domains}
          goals={goals}
          initialEditMode={modalEditMode}
          onClose={() => setModalOpen(false)}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </>
  )
}

export default TaskItem
