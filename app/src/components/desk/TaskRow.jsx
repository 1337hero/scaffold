import { colorClass } from "@/data/mock.js"
import { Checkbox } from "@/components/ui/Checkbox.jsx"

export function TaskRow({ task, onToggle }) {
  return (
    <div
      class="surface-card py-4 px-5 flex items-center gap-3.5 mb-1 cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber/70"
      onClick={onToggle}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault()
          onToggle()
        }
      }}
    >
      <span class="font-mono text-[0.75rem] text-text-muted min-w-[16px]">{task.num}</span>

      <Checkbox
        checked={task.done}
        onChange={(e) => { e.stopPropagation(); onToggle() }}
      />

      <div>
        <div class={`text-[0.92rem] font-semibold ${task.done ? 'line-through text-text-dim' : ''}`}>{task.title}</div>
        <div class={`text-[0.7rem] font-medium uppercase tracking-[0.06em] ${colorClass(task.project.color)}`}>
          {task.project.name}
        </div>
      </div>
    </div>
  )
}
