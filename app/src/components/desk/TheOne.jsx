import { cn } from "@/lib/utils.js"
import { colorClass } from "@/data/mock.js"
import { Checkbox } from "@/components/ui/Checkbox.jsx"

export function TheOne({ task, onToggle, onToggleStep }) {
  return (
    <div class="bg-surface border border-border border-l-[3px] border-l-amber rounded-xl p-7 mb-3">
      <div class="text-[0.76rem] font-semibold uppercase tracking-[0.12em] text-amber mb-3">
        The One
      </div>

      <div class="flex items-start gap-4">
        <div class="mt-0.5">
          <Checkbox checked={task.done} onChange={onToggle} />
        </div>

        <div class="flex-1">
          <div class={cn("text-[1.22rem] font-semibold mb-1", task.done && "line-through text-text-dim")}>
            {task.title}
          </div>
          <div class={`text-[0.8rem] font-medium uppercase tracking-[0.08em] ${colorClass(task.project.color)}`}>
            {task.project.name}
          </div>

          {task.microSteps?.length > 0 && (
            <div class="mt-4.5 pt-4.5 border-t border-border">
              <div class="text-[0.74rem] uppercase tracking-[0.08em] text-text-muted mb-2">Steps</div>
              {task.microSteps.map((step) => (
                <div key={step.id} class="flex items-center gap-3 py-2">
                  <Checkbox
                    checked={step.done}
                    onChange={() => onToggleStep(step.id)}
                    size="sm"
                  />
                  <span class={cn(
                    "font-mono text-[0.9rem] text-text-dim",
                    step.done && "line-through opacity-40"
                  )}>
                    {step.text}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
