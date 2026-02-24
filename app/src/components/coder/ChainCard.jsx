import { useMutation, useQueryClient } from "@tanstack/react-query"
import StepLog from "./StepLog.jsx"
import StepPipeline from "./StepPipeline.jsx"

const formatElapsed = (secs) => {
  const s = Math.floor(secs)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const rem = s % 60
  return `${m}m ${rem.toString().padStart(2, "0")}s`
}

const statusStyles = {
  running:   { dot: "bg-[#C47D3A] animate-pulse", label: "running",   cls: "text-[#C47D3A]" },
  done:      { dot: "bg-[#4ADE80]",               label: "done",       cls: "text-[#4ADE80]" },
  failed:    { dot: "bg-[#F87171]",               label: "failed",     cls: "text-[#F87171]" },
  cancelled: { dot: "bg-[#6B5F52]",               label: "cancelled",  cls: "text-[#6B5F52]" },
}

const ChainCard = ({ task, stepLogs = {}, currentAction }) => {
  const queryClient = useQueryClient()
  const isRunning = task.status === "running"

  const killMutation = useMutation({
    mutationFn: () =>
      fetch(`/api/coder/tasks/${task.id}`, {
        method: "DELETE",
        credentials: "include",
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["coder-tasks"] }),
  })

  const elapsed = task.ended_at
    ? (new Date(task.ended_at) - new Date(task.started_at)) / 1000
    : (Date.now() - new Date(task.started_at)) / 1000

  const style = statusStyles[task.status] || statusStyles.running

  return (
    <div class="border border-[#2A2318] rounded-2xl p-5 bg-[#18140F] mb-4">
      {/* Header */}
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-2">
          <span class={`w-2 h-2 rounded-full shrink-0 ${style.dot}`} />
          <span class={`text-xs font-mono ${style.cls}`}>{style.label}</span>
          <span class="text-xs text-[#5A4F42] font-mono ml-1">
            [{formatElapsed(elapsed)}]
          </span>
          {task.status === "failed" && task.failed_step && (
            <span class="text-xs text-[#F87171] font-mono">at {task.failed_step}</span>
          )}
        </div>
        {isRunning && (
          <button
            type="button"
            onClick={() => killMutation.mutate()}
            disabled={killMutation.isPending}
            class="text-[10px] font-mono px-2 py-1 border border-[#5A2E2E] text-[#F87171] rounded hover:bg-[#3A1E1E] transition-colors"
          >
            Kill
          </button>
        )}
      </div>

      {/* Task description */}
      <p class="text-[#F5F0E8] text-sm font-medium mb-1 leading-snug">
        &ldquo;{task.task}&rdquo;
      </p>
      <p class="text-[10px] text-[#5A4F42] font-mono mb-3">{task.cwd}</p>

      {/* Step pipeline */}
      <StepPipeline steps={task.steps || []} />

      {/* Current action strip (running only) */}
      {isRunning && currentAction && (
        <div class="flex items-center gap-2 mt-3 px-3 py-2 bg-[#2E2318] border border-[#C47D3A]/20 rounded-lg">
          <span class="w-3 h-3 border-2 border-[#C47D3A]/40 border-t-[#C47D3A] rounded-full animate-spin shrink-0" />
          <span class="text-[11px] text-[#C47D3A] font-mono truncate">
            {currentAction.tool && (
              <span class="opacity-60 mr-1">{currentAction.tool}</span>
            )}
            {currentAction.input}
          </span>
        </div>
      )}

      {/* Summary (done) */}
      {task.status === "done" && task.summary && (
        <p class="mt-3 text-xs text-[#9C8E7A] leading-relaxed line-clamp-3">
          {task.summary}
        </p>
      )}

      {/* Error (failed) */}
      {task.status === "failed" && task.error && (
        <p class="mt-3 text-xs text-[#F87171] font-mono leading-relaxed">
          {task.error}
        </p>
      )}

      {/* Per-step logs */}
      {(task.steps || []).some((s) => (stepLogs[s.name] || []).length > 0) && (
        <div class="mt-3 space-y-1">
          {(task.steps || []).map((step) => (
            <StepLog
              key={step.name}
              step={step}
              events={stepLogs[step.name] || []}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default ChainCard
