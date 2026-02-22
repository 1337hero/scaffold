import { colorClass } from "@/constants/colors.js"
import { Checkbox } from "@/components/ui/Checkbox.jsx"

export function TaskRow({ task, onToggle }) {
  return (
    <button
      type="button"
      class="surface-card py-5 px-6 flex items-center gap-4 mb-2 cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber/70 text-left w-full"
      onClick={onToggle}
    >
      <span class="font-mono text-[0.84rem] text-text-muted min-w-[18px]">{task.num}</span>

      <Checkbox
        checked={task.done}
        onChange={(e) => { e.stopPropagation(); onToggle() }}
      />

      <div>
        <div class={`text-[1.02rem] font-semibold ${task.done ? 'line-through text-text-dim' : ''}`}>{task.title}</div>
        <div class={`text-[0.8rem] font-medium uppercase tracking-[0.08em] ${colorClass(task.project.color)}`}>
          {task.project.name}
        </div>
      </div>
    </button>
  )
}
