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

const borderColor = {
  running: "border-l-[#C47D3A]",
  done: "border-l-[#5A9E6F]",
  failed: "border-l-[#C4617A]",
  cancelled: "border-l-app-muted",
}

const ChainCard = ({ task, stepLogs = {}, currentAction, stepProgress }) => {
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

  const leftBorder = borderColor[task.status] || ""

  return (
    <div class={`bg-card-bg border border-app-border rounded-[20px] card-shadow p-5 px-6 mb-3.5 overflow-hidden border-l-3 ${leftBorder}`}>
      {/* Header */}
      <div class="flex items-center gap-2.5 mb-2.5">
        {task.status === "running" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-[#C47D3A]">
            <span class="pulse-dot" />
            running
          </div>
        )}
        {task.status === "done" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-[#5A9E6F]">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12" /></svg>
            done
          </div>
        )}
        {task.status === "failed" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-[#C4617A]">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
            failed
            {task.failed_step && <> · {task.failed_step}</>}
          </div>
        )}
        {task.status === "cancelled" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-app-muted">cancelled</div>
        )}

        <span class="font-mono text-[11px] text-app-muted">{formatElapsed(elapsed)}</span>

        {isRunning && (
          <button
            type="button"
            onClick={() => killMutation.mutate()}
            disabled={killMutation.isPending}
            class="ml-auto px-2.5 py-0.5 border border-[#C4617A]/20 rounded-md bg-transparent text-[#C4617A]/55 font-mono text-[10px] uppercase tracking-wide cursor-pointer hover:bg-[#C4617A]/5 hover:text-[#C4617A]/90 transition-all"
          >
            Kill
          </button>
        )}
      </div>

      {/* Task description */}
      <p class="text-sm font-medium text-app-ink mb-1 leading-snug">
        &ldquo;{task.task}&rdquo;
      </p>
      <p class="text-[10px] text-app-muted opacity-55 font-mono mb-4">
        {task.cwd}
        {task.chain && <> · {task.chain} chain</>}
      </p>

      {/* Step pipeline */}
      <StepPipeline steps={task.steps || []} />

      {/* Current action strip (running only) */}
      {isRunning && currentAction && (
        <div class="flex items-center gap-2.5 px-3.5 py-2.5 bg-[#C47D3A]/5 border border-[#C47D3A]/15 rounded-[10px] mb-3.5">
          <span class="w-[13px] h-[13px] border-2 border-[#C47D3A]/20 border-t-[#C47D3A] rounded-full animate-spin shrink-0" />
          <span class="coder-action-text font-mono text-xs text-[#C47D3A] truncate">
            {currentAction.input}
          </span>
          <span class="ml-auto font-mono text-[9px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-[#C47D3A]/8 text-[#C47D3A] opacity-70">
            {currentAction.step}
          </span>
        </div>
      )}

      {/* Summary (done) */}
      {task.status === "done" && task.summary && (
        <div class="flex items-center gap-2 px-3.5 py-2.5 rounded-[10px] mt-1 bg-[#5A9E6F]/6 border border-[#5A9E6F]/15 font-mono text-xs text-[#5A9E6F]">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" class="shrink-0"><polyline points="20 6 9 17 4 12" /></svg>
          <span class="line-clamp-2">{task.summary}</span>
        </div>
      )}

      {/* Error (failed) */}
      {task.status === "failed" && task.error && (
        <div class="flex items-center gap-2 px-3.5 py-2.5 rounded-[10px] mt-1 bg-[#C4617A]/6 border border-[#C4617A]/15 font-mono text-xs text-[#C4617A]">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" class="shrink-0"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
          <span class="line-clamp-2">{task.error}</span>
        </div>
      )}

      {/* Per-step logs */}
      {(task.steps || []).some((s) => (stepLogs[s.name] || []).length > 0) && (
        <div class="mt-3.5">
          <StepLog
            steps={task.steps || []}
            stepLogs={stepLogs}
          />
        </div>
      )}
    </div>
  )
}

export default ChainCard
