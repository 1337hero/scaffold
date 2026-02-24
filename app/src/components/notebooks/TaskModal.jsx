import { useState, useEffect } from "preact/hooks"
import { nullable } from "@/utils/nullable.js"

const PRIORITY_OPTIONS = ["low", "normal", "high", "urgent"]

const PRIORITY_COLORS = {
  low: "bg-gray-100 text-gray-500",
  normal: "bg-blue-100 text-blue-600",
  high: "bg-orange-100 text-orange-600",
  urgent: "bg-red-100 text-red-600",
}

const TaskModal = ({ task, domains = [], goals = [], onClose, onSave, onDelete }) => {
  const [title, setTitle] = useState(task?.Title ?? "")
  const [dueDate, setDueDate] = useState(nullable(task?.DueDate) ?? "")
  const [priority, setPriority] = useState(task?.Priority ?? "normal")
  const [domainId, setDomainId] = useState(task?.DomainID ?? "")
  const [goalId, setGoalId] = useState(nullable(task?.GoalID) ?? "")
  const [recurring, setRecurring] = useState(nullable(task?.Recurring) ?? "")
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const filteredGoals = goals.filter((g) => domainId && g.DomainID === Number(domainId))


  useEffect(() => {
    document.body.style.overflow = "hidden"
    return () => { document.body.style.overflow = "" }
  }, [])

  useEffect(() => {
    const handleKey = (e) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onClose])

  function handleDomainChange(newDomainId) {
    setDomainId(newDomainId)
    setGoalId("")
  }

  function handleSave() {
    const payload = {
      title: title.trim(),
      due_date: dueDate || null,
      priority,
      domain_id: domainId ? Number(domainId) : null,
      goal_id: goalId || null,
      recurring: recurring.trim() || null,
    }
    if (task) {
      onSave(task.ID, payload)
    } else {
      onSave(payload)
    }
    onClose()
  }

  function handleDelete() {
    onDelete(task.ID)
    onClose()
  }

  function handleCancel() {
    onClose()
  }

  const selectClass = "bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm outline-none focus:border-app-ink/30 transition-all"
  const inputClass = "w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 text-sm outline-none focus:border-app-ink/30 transition-all"

  return (
    <div
      class="fixed inset-0 bg-black/50 z-50 flex items-center justify-center"
      onClick={onClose}
    >
      <div
        class="w-[80vw] max-h-[80vh] rounded-3xl bg-[var(--color-card-bg)] border border-app-border card-shadow flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div class="p-6 pb-0 flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            <input
              type="text"
              value={title}
              onInput={(e) => setTitle(e.currentTarget.value)}
              class="w-full text-xl font-bold bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all"
              placeholder="Title"
              autoFocus
            />
          </div>
          <div class="flex items-center gap-3 shrink-0">
            <button type="button" onClick={onClose}
              class="text-app-muted hover:text-app-ink transition-colors cursor-pointer">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 6 6 18" /><path d="m6 6 12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Body */}
        <div class="p-6 flex-1 overflow-y-auto">
          <div class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-1">
                <label class="text-[10px] mono uppercase font-bold text-app-muted">Due Date</label>
                <input type="date" value={dueDate}
                  onInput={(e) => setDueDate(e.currentTarget.value)}
                  class={inputClass} />
              </div>
              <div class="space-y-1">
                <label class="text-[10px] mono uppercase font-bold text-app-muted">Priority</label>
                <select value={priority} onChange={(e) => setPriority(e.currentTarget.value)}
                  class={selectClass + " w-full"}>
                  {PRIORITY_OPTIONS.map((p) => (
                    <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>
                  ))}
                </select>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-1">
                <label class="text-[10px] mono uppercase font-bold text-app-muted">Domain</label>
                <select value={domainId} onChange={(e) => handleDomainChange(e.currentTarget.value)}
                  class={selectClass + " w-full"}>
                  <option value="">None</option>
                  {domains.map((d) => (
                    <option key={d.ID} value={d.ID}>{d.Name}</option>
                  ))}
                </select>
              </div>
              <div class="space-y-1">
                <label class="text-[10px] mono uppercase font-bold text-app-muted">Goal</label>
                <select value={goalId} onChange={(e) => setGoalId(e.currentTarget.value)}
                  class={selectClass + " w-full"} disabled={!domainId}>
                  <option value="">None</option>
                  {filteredGoals.map((g) => (
                    <option key={g.ID} value={g.ID}>{g.Title}</option>
                  ))}
                </select>
              </div>
            </div>
            <div class="space-y-1">
              <label class="text-[10px] mono uppercase font-bold text-app-muted">Recurring</label>
              <input type="text" value={recurring}
                onInput={(e) => setRecurring(e.currentTarget.value)}
                class={inputClass} placeholder="e.g. daily, weekly" />
            </div>
          </div>
        </div>

        {/* Footer */}
        <div class="p-6 pt-0 flex items-center justify-between">
          <div>
            {onDelete && task && !confirmingDelete && (
              <button type="button" onClick={() => setConfirmingDelete(true)}
                class="text-[10px] mono uppercase font-bold text-red-400 hover:text-red-600 transition-colors cursor-pointer">
                Delete
              </button>
            )}
            {confirmingDelete && (
              <div class="flex items-center gap-2 text-[10px] mono uppercase font-bold">
                <span class="text-red-500">Delete this task?</span>
                <button type="button" onClick={handleDelete}
                  class="text-red-600 hover:text-red-800 transition-colors cursor-pointer">
                  Confirm
                </button>
                <button type="button" onClick={() => setConfirmingDelete(false)}
                  class="text-app-muted hover:text-app-ink transition-colors cursor-pointer">
                  Cancel
                </button>
              </div>
            )}
          </div>
          <div class="flex items-center gap-2">
            <button type="button" onClick={handleCancel}
              class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all cursor-pointer">
              Cancel
            </button>
            <button type="button" onClick={handleSave} disabled={!title.trim()}
              class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer">
              Save
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default TaskModal
