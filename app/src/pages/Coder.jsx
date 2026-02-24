import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "preact/hooks"
import ChainCard from "@/components/coder/ChainCard.jsx"

const coderTasksQuery = {
  queryKey: ["coder-tasks"],
  queryFn: () =>
    fetch("/api/coder/tasks", { credentials: "include" }).then((r) => r.json()),
  refetchInterval: 10_000,
}

const Coder = () => {
  const queryClient = useQueryClient()
  const { data: rawTasks } = useQuery(coderTasksQuery)
  const tasks = rawTasks ?? []

  // stepLogs: { taskId: { stepName: [event, ...] } }
  const [stepLogs, setStepLogs] = useState({})
  // currentActions: { taskId: { step, tool, input } }
  const [currentActions, setCurrentActions] = useState({})

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
      queryClient.invalidateQueries({ queryKey: ["coder-tasks"] })
    })

    source.addEventListener("chain_failed", (e) => {
      const data = JSON.parse(e.data)
      setCurrentActions((prev) => {
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
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default Coder
