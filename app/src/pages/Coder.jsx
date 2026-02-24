import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "preact/hooks"
import ChainCard from "@/components/coder/ChainCard.jsx"
import { agentChainsQuery, dispatchAgentTask, fetchStepEvents } from "@/api/queries.js"

const agentTasksQuery = {
  queryKey: ["agent-tasks"],
  queryFn: () =>
    fetch("/api/agents/tasks", { credentials: "include" }).then((r) => r.json()),
  refetchInterval: 10_000,
}

const Coder = () => {
  const queryClient = useQueryClient()
  const { data: rawTasks, isError, error } = useQuery(agentTasksQuery)
  const { data: chainOptions = [], isLoading: chainsLoading } = useQuery(agentChainsQuery)
  const tasks = rawTasks ?? []

  // stepLogs: { taskId: { stepName: [event, ...] } }
  const [stepLogs, setStepLogs] = useState({})
  // currentActions: { taskId: { step, tool, input } }
  const [currentActions, setCurrentActions] = useState({})
  // stepProgress: { taskId: { num, total, step } }
  const [stepProgress, setStepProgress] = useState({})

  // dispatch form state
  const [dispatchTask, setDispatchTask] = useState("")
  const [dispatchChain, setDispatchChain] = useState("")
  const [dispatchCwd, setDispatchCwd] = useState("")

  useEffect(() => {
    if (chainOptions.length && !dispatchChain) {
      setDispatchChain(chainOptions[0])
    }
  }, [chainOptions])

  const dispatchMutation = useMutation({
    mutationFn: (params) => dispatchAgentTask(params),
    onSuccess: () => {
      setDispatchTask("")
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    },
  })

  useEffect(() => {
    const source = new EventSource("/api/agents/stream")

    const appendLog = (taskId, step, event) => {
      setStepLogs((prev) => ({
        ...prev,
        [taskId]: {
          ...(prev[taskId] || {}),
          [step]: [...((prev[taskId] || {})[step] || []), event],
        },
      }))
    }

    source.addEventListener("chain_started", (e) => {
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    })

    source.addEventListener("step_started", (e) => {
      const data = JSON.parse(e.data)
      setStepProgress(prev => ({
        ...prev,
        [data.task_id]: { num: data.step_num, total: data.step_total, step: data.step }
      }))
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    })

    source.addEventListener("step_event", (e) => {
      const data = JSON.parse(e.data)
      appendLog(data.task_id, data.step, data)

      if (data.type === "tool_use") {
        setCurrentActions((prev) => ({
          ...prev,
          [data.task_id]: { step: data.step, tool: data.tool, input: data.input },
        }))
      }
    })

    source.addEventListener("step_done", (e) => {
      const data = JSON.parse(e.data)
      setCurrentActions((prev) => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    })

    source.addEventListener("chain_done", (e) => {
      const data = JSON.parse(e.data)
      setStepProgress(prev => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    })

    source.addEventListener("chain_failed", (e) => {
      const data = JSON.parse(e.data)
      setCurrentActions((prev) => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      setStepProgress(prev => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      queryClient.invalidateQueries({ queryKey: ["agent-tasks"] })
    })

    source.onerror = () => {
      // EventSource auto-reconnects
    }

    return () => source.close()
  }, [queryClient])

  // Hydrate step logs from disk for tasks that have steps (restores logs after reload)
  useEffect(() => {
    if (!tasks.length) return

    tasks.forEach(task => {
      if (!task.steps?.length) return
      if (stepLogs[task.id] && Object.keys(stepLogs[task.id]).length > 0) return

      task.steps.forEach((step, i) => {
        if (step.status === "pending") return
        const stepNum = i + 1
        fetchStepEvents(task.id, String(stepNum))
          .then(events => {
            if (!events?.length) return
            setStepLogs(prev => ({
              ...prev,
              [task.id]: {
                ...(prev[task.id] || {}),
                [step.name]: events,
              },
            }))
          })
          .catch(() => {})
      })
    })
  }, [tasks.length])

  const runningTask = tasks.find((t) => t.status === "running")

  return (
    <div>
      <div class="mb-9">
        <h2 class="text-[28px] font-bold tracking-tight">Agents</h2>
        <p class="text-sm text-app-muted mt-1">
          Autonomous chains dispatched from Signal or the web
        </p>
      </div>

      {isError && (
        <div class="border border-status-error/20 rounded-[14px] p-3 mb-4 bg-status-error/5">
          <p class="text-status-error text-xs font-mono">
            Failed to load tasks: {error?.message || "unknown error"}
          </p>
        </div>
      )}

      {/* Status bar */}
      <div class="flex items-center gap-3.5 px-[18px] py-3 bg-card-bg border border-app-border rounded-[14px] card-shadow mb-8">
        {runningTask ? (
          <>
            <span class="pulse-dot" />
            <span class="font-mono text-[11px] text-status-running font-medium">
              1 chain running
            </span>
            <span class="text-app-border">·</span>
            <span class="font-mono text-[11px] text-app-muted">
              {runningTask.chain}
              {stepProgress[runningTask.id] && ` · ${stepProgress[runningTask.id].step} step`}
              {` · ${runningTask.cwd}`}
            </span>
          </>
        ) : (
          <>
            <span class="w-2 h-2 rounded-full bg-app-muted shrink-0" />
            <span class="font-mono text-[11px] text-app-muted">idle</span>
            <span class="text-app-border">·</span>
            <span class="font-mono text-[11px] text-app-muted">ready · no active chains</span>
          </>
        )}
      </div>

      {/* Dispatch form */}
      <div class="bg-card-bg border border-app-border rounded-[20px] card-shadow p-5 mb-6">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (!dispatchTask.trim()) return
            dispatchMutation.mutate({ task: dispatchTask.trim(), chain: dispatchChain, cwd: dispatchCwd })
          }}
          class="flex flex-col gap-3"
        >
          <div class="flex gap-2">
            <input
              type="text"
              value={dispatchTask}
              onInput={(e) => setDispatchTask(e.currentTarget.value)}
              placeholder="Describe the task..."
              class="flex-1 bg-input-bg border border-app-border rounded-lg px-3 py-2 text-sm text-app-ink font-mono placeholder:text-app-muted/40 outline-none focus:border-accent/40"
            />
            <select
              value={dispatchChain}
              onChange={(e) => setDispatchChain(e.currentTarget.value)}
              class="bg-input-bg border border-app-border rounded-lg px-3 py-2 text-[11px] font-mono text-app-muted outline-none cursor-pointer"
            >
              {chainOptions.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input
              type="text"
              value={dispatchCwd}
              onInput={(e) => setDispatchCwd(e.currentTarget.value)}
              placeholder="Working directory (optional, defaults to scaffold root)"
              class="flex-1 bg-input-bg border border-app-border rounded-lg px-3 py-2 text-[11px] font-mono text-app-muted placeholder:text-app-muted/40 outline-none focus:border-accent/40"
            />
            <button
              type="submit"
              disabled={!dispatchTask.trim() || !dispatchChain || chainsLoading || dispatchMutation.isPending}
              class="px-4 py-2 rounded-lg bg-accent text-white text-[11px] font-mono font-semibold hover:bg-accent-hover transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
            >
              {dispatchMutation.isPending ? "Sending..." : "Dispatch"}
            </button>
          </div>
          {dispatchMutation.isError && (
            <p class="text-[11px] text-status-error font-mono">
              {dispatchMutation.error?.message || "dispatch failed"}
            </p>
          )}
          {dispatchMutation.isSuccess && (
            <p class="text-[11px] text-status-done font-mono">Dispatched</p>
          )}
        </form>
      </div>

      {/* Section header */}
      <div class="flex items-center gap-2 mb-[18px]">
        <span class="font-mono text-[10px] uppercase tracking-[0.12em] opacity-40 whitespace-nowrap">Tasks</span>
        <div class="h-px flex-1 bg-app-border" />
      </div>

      {tasks.length === 0 ? (
        <div class="py-16 text-center opacity-35">
          <p class="font-serif italic text-base">No chains running</p>
          <span class="font-mono text-[11px] block mt-1.5">
            Dispatch a task from Signal or the form above
          </span>
        </div>
      ) : (
        <div>
          {tasks.map((task) => (
            <ChainCard
              key={task.id}
              task={task}
              stepLogs={stepLogs[task.id] || {}}
              currentAction={currentActions[task.id]}
              stepProgress={stepProgress[task.id]}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default Coder
