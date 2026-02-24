import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "preact/hooks"
import ChainCard from "@/components/coder/ChainCard.jsx"
import { dispatchCoderTask, fetchStepEvents } from "@/api/queries.js"

const coderTasksQuery = {
  queryKey: ["coder-tasks"],
  queryFn: () =>
    fetch("/api/coder/tasks", { credentials: "include" }).then((r) => r.json()),
  refetchInterval: 10_000,
}

const Coder = () => {
  const queryClient = useQueryClient()
  const { data: rawTasks, isError, error } = useQuery(coderTasksQuery)
  const tasks = rawTasks ?? []

  // stepLogs: { taskId: { stepName: [event, ...] } }
  const [stepLogs, setStepLogs] = useState({})
  // currentActions: { taskId: { step, tool, input } }
  const [currentActions, setCurrentActions] = useState({})
  // stepProgress: { taskId: { num, total, step } }
  const [stepProgress, setStepProgress] = useState({})

  // dispatch form state
  const [dispatchTask, setDispatchTask] = useState("")
  const [dispatchChain, setDispatchChain] = useState("single")
  const [dispatchCwd, setDispatchCwd] = useState("")

  const dispatchMutation = useMutation({
    mutationFn: (params) => dispatchCoderTask(params),
    onSuccess: () => {
      setDispatchTask("")
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
    },
  })

  useEffect(() => {
    const source = new EventSource("/api/coder/stream")

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
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
    })

    source.addEventListener("step_started", (e) => {
      const data = JSON.parse(e.data)
      setStepProgress(prev => ({
        ...prev,
        [data.task_id]: { num: data.step_num, total: data.step_total, step: data.step }
      }))
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
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
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
    })

    source.addEventListener("chain_done", (e) => {
      const data = JSON.parse(e.data)
      setStepProgress(prev => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
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
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
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

  return (
    <div>
      <div class="mb-8">
        <h2 class="text-2xl font-serif italic text-[#F5F0E8] font-semibold">
          Agent Activity
        </h2>
        <p class="text-sm text-[#6B5F52] mt-1">
          Code chains dispatched via Signal. Live step output below.
        </p>
      </div>

      {isError && (
        <div class="border border-[#5A2E2E] rounded-2xl p-5 mb-4 bg-[#2A1A1A]">
          <p class="text-[#F87171] text-sm font-mono">
            Failed to load tasks: {error?.message || "unknown error"}
          </p>
        </div>
      )}

      <div class="border border-[#2A2318] rounded-2xl p-5 bg-[#18140F] mb-6">
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
              class="flex-1 bg-[#0D0A07] border border-[#2A2318] rounded-lg px-3 py-2 text-sm text-[#F5F0E8] font-mono placeholder:text-[#3A3228] outline-none focus:border-[#C47D3A]/40"
            />
            <select
              value={dispatchChain}
              onChange={(e) => setDispatchChain(e.currentTarget.value)}
              class="bg-[#0D0A07] border border-[#2A2318] rounded-lg px-3 py-2 text-[11px] font-mono text-[#9C8E7A] outline-none cursor-pointer"
            >
              <option value="single">single</option>
              <option value="fix">fix</option>
              <option value="implement">implement</option>
              <option value="spec">spec</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input
              type="text"
              value={dispatchCwd}
              onInput={(e) => setDispatchCwd(e.currentTarget.value)}
              placeholder="Working directory (optional, defaults to scaffold root)"
              class="flex-1 bg-[#0D0A07] border border-[#2A2318] rounded-lg px-3 py-2 text-[11px] font-mono text-[#9C8E7A] placeholder:text-[#3A3228] outline-none focus:border-[#C47D3A]/40"
            />
            <button
              type="submit"
              disabled={!dispatchTask.trim() || dispatchMutation.isPending}
              class="px-4 py-2 rounded-lg bg-[#C47D3A]/10 text-[#C47D3A] text-[11px] font-mono font-semibold hover:bg-[#C47D3A]/20 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
            >
              {dispatchMutation.isPending ? "Sending..." : "Dispatch"}
            </button>
          </div>
          {dispatchMutation.isError && (
            <p class="text-[11px] text-[#F87171] font-mono">
              {dispatchMutation.error?.message || "dispatch failed"}
            </p>
          )}
          {dispatchMutation.isSuccess && (
            <p class="text-[11px] text-[#4ADE80] font-mono">Dispatched</p>
          )}
        </form>
      </div>

      {tasks.length === 0 ? (
        <div class="border border-[#2A2318] rounded-2xl p-8 text-center">
          <p class="text-[#5A4F42] text-sm">No tasks yet.</p>
          <p class="text-[#3A3228] text-xs mt-2 font-mono">
            Say "implement issue #47" via Signal to start a chain.
          </p>
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
