import { cn } from "@/lib/utils.js"
import { colorClass } from "@/data/mock.js"
import { Checkbox } from "@/components/ui/Checkbox.jsx"

export function TheOne({ task, onToggle, onToggleStep }) {
  return (
    <div class="bg-surface border border-border border-l-[3px] border-l-amber rounded-lg p-6 mb-2">
      <div class="text-[0.65rem] font-semibold uppercase tracking-[0.12em] text-amber mb-2.5">
        The One
      </div>

      <div class="flex items-start gap-3.5">
        <div class="mt-0.5">
          <Checkbox checked={task.done} onChange={onToggle} />
        </div>

        <div class="flex-1">
          <div class="text-[1.05rem] font-semibold mb-0.5">{task.title}</div>
          <div class={`text-[0.7rem] font-medium uppercase tracking-[0.06em] ${colorClass(task.project.color)}`}>
            {task.project.name}
          </div>

          {task.microSteps?.length > 0 && (
            <div class="mt-3.5 pt-3.5 border-t border-border">
              <div class="text-[0.65rem] uppercase tracking-[0.08em] text-text-muted mb-1.5">Steps</div>
              {task.microSteps.map((step) => (
                <div key={step.id} class="flex items-center gap-2.5 py-[5px]">
                  <Checkbox
                    checked={step.done}
                    onChange={() => onToggleStep(step.id)}
                    size="sm"
                  />
                  <span class={cn(
                    "font-mono text-[0.8rem] text-text-dim",
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
