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
  running: "chain-border-running",
  done: "chain-border-done",
  failed: "chain-border-failed",
  cancelled: "chain-border-cancelled",
}

const ChainCard = ({ task, stepLogs = {}, currentAction, stepProgress }) => {
  const queryClient = useQueryClient()
  const isRunning = task.status === "running"

  const killMutation = useMutation({
    mutationFn: () =>
      fetch(`/api/agents/tasks/${task.id}`, {
        method: "DELETE",
        credentials: "include",
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["agent-tasks"] }),
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
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-status-running">
            <span class="pulse-dot" />
            running
          </div>
        )}
        {task.status === "done" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-status-done">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12" /></svg>
            done
          </div>
        )}
        {task.status === "failed" && (
          <div class="flex items-center gap-1.5 font-mono text-[11px] text-status-failed">
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
            class="ml-auto px-2.5 py-0.5 border border-status-error/20 rounded-md bg-transparent text-status-error/55 font-mono text-[10px] uppercase tracking-wide cursor-pointer hover:bg-status-error/5 hover:text-status-error/90 transition-all"
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
        <div class="flex items-center gap-2.5 px-3.5 py-2.5 bg-accent/5 border border-accent/15 rounded-[10px] mb-3.5">
          <span class="w-[13px] h-[13px] border-2 border-accent/20 border-t-accent rounded-full animate-spin shrink-0" />
          <span class="coder-action-text font-mono text-xs text-status-running truncate">
            {currentAction.input}
          </span>
          <span class="ml-auto font-mono text-[9px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-accent/8 text-status-running opacity-70">
            {currentAction.step}
          </span>
        </div>
      )}

      {/* Summary (done) */}
      {task.status === "done" && task.summary && (
        <div class="flex items-center gap-2 px-3.5 py-2.5 rounded-[10px] mt-1 bg-status-done/6 border border-status-done/15 font-mono text-xs text-status-done">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" class="shrink-0"><polyline points="20 6 9 17 4 12" /></svg>
          <span class="line-clamp-2">{task.summary}</span>
        </div>
      )}

      {/* Error (failed) */}
      {task.status === "failed" && task.error && (
        <div class="flex items-center gap-2 px-3.5 py-2.5 rounded-[10px] mt-1 bg-status-error/6 border border-status-error/15 font-mono text-xs text-status-error">
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
