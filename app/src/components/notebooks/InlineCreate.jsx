import { useState } from "preact/hooks"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { createTask, createNote, createGoal } from "@/api/queries.js"

const InlineCreate = ({ domainId, type, compact }) => {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState("")
  const [dueDate, setDueDate] = useState("")
  const [content, setContent] = useState("")
  const [goalType, setGoalType] = useState("binary")
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (data) => {
      if (type === "task") return createTask(data)
      if (type === "note") return createNote(data)
      if (type === "goal") return createGoal(data)
    },
    onSuccess: () => {
      if (type === "task") queryClient.invalidateQueries({ queryKey: ["tasks"] })
      if (type === "note") queryClient.invalidateQueries({ queryKey: ["notes"] })
      if (type === "goal") queryClient.invalidateQueries({ queryKey: ["goals"] })
      setTitle("")
      setDueDate("")
      setContent("")
      setGoalType("binary")
      setOpen(false)
    },
  })

  function handleSubmit(e) {
    e.preventDefault()
    const trimmed = title.trim()
    if (!trimmed || mutation.isPending) return
    const base = { title: trimmed, domain_id: Number(domainId) }
    if (type === "task") mutation.mutate({ ...base, ...(dueDate && { due_date: dueDate }) })
    else if (type === "note") mutation.mutate({ ...base, ...(content && { content }) })
    else if (type === "goal") mutation.mutate({ ...base, type: goalType })
  }

  const label = type.charAt(0).toUpperCase() + type.slice(1)
  const inputClass =
    "bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm outline-none focus:border-app-ink/30 transition-all w-full"

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        class="text-[10px] mono uppercase font-bold text-emerald-600 hover:underline cursor-pointer"
      >
        + Add {label}
      </button>
    )
  }

  return (
    <form
      onSubmit={handleSubmit}
      class="p-4 bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow flex flex-col gap-2.5"
    >
      <input
        type="text"
        value={title}
        onInput={(e) => setTitle(e.currentTarget.value)}
        class={inputClass}
        placeholder={`${label} title...`}
        autoFocus
      />

      {type === "task" && (
        <input
          type="date"
          value={dueDate}
          onInput={(e) => setDueDate(e.currentTarget.value)}
          class={inputClass}
        />
      )}

      {type === "goal" && (
        <select
          value={goalType}
          onChange={(e) => setGoalType(e.currentTarget.value)}
          class={inputClass + " appearance-none cursor-pointer"}
        >
          <option value="binary">Binary</option>
          <option value="measurable">Measurable</option>
          <option value="habit">Habit</option>
        </select>
      )}

      {type === "note" && (
        <input
          type="text"
          value={content}
          onInput={(e) => setContent(e.currentTarget.value)}
          class={inputClass}
          placeholder="Content (optional)..."
        />
      )}

      <div class="flex items-center gap-2 justify-end">
        <button
          type="button"
          onClick={() => { setOpen(false); setTitle("") }}
          class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={!title.trim() || mutation.isPending}
          class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
        >
          {mutation.isPending ? "Adding..." : `Add ${label}`}
        </button>
      </div>
    </form>
  )
}

export default InlineCreate
