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

const TaskItem = ({ task, onComplete }) => {
  const done = task.Status === "done"

  return (
    <div class="p-4 flex items-center gap-4 group">
      <button
        type="button"
        onClick={() => !done && onComplete?.()}
        class={`transition-colors shrink-0 ${done ? "text-emerald-500 cursor-default" : "text-app-muted hover:text-app-ink cursor-pointer"}`}
      >
        {done ? <CheckCircleIcon /> : <CircleIcon />}
      </button>
      <span class={`text-sm font-medium flex-1 ${done ? "line-through opacity-40" : ""}`}>
        {task.Title}
      </span>

    </div>
  )
}

export default TaskItem
