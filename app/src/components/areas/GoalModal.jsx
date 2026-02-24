import { useState, useEffect } from "preact/hooks"
import { nullable } from "@/utils/nullable.js"

const DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"]
const STATUS_OPTIONS = ["active", "paused", "completed", "abandoned"]

function parseScheduleDays(goal) {
  try { return JSON.parse(goal.ScheduleDays?.String || "[]") } catch { return [] }
}

const GoalModal = ({ goal, domains, tasks, onClose, onSave, onDelete }) => {
  const [title, setTitle] = useState(goal.Title ?? "")
  const [status, setStatus] = useState(goal.Status ?? "active")
  const [targetDate, setTargetDate] = useState(nullable(goal.DueDate) ?? "")
  const [domainId, setDomainId] = useState(nullable(goal.DomainID) ?? "")
  const [currentValue, setCurrentValue] = useState(goal.CurrentValue?.Float64 ?? 0)
  const [targetValue, setTargetValue] = useState(goal.TargetValue?.Float64 ?? 0)
  const [scheduleDays, setScheduleDays] = useState(parseScheduleDays(goal))
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const type = goal.Type || "binary"
  const habitType = goal.HabitType?.String

  const openTaskCount = (tasks || []).filter(
    t => t.GoalID?.String === goal.ID && t.Status !== "done" && t.Status !== "deleted"
  ).length

  useEffect(() => {
    document.body.style.overflow = "hidden"
    return () => { document.body.style.overflow = "" }
  }, [])

  useEffect(() => {
    const handleKey = (e) => { if (e.key === "Escape") onClose() }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onClose])

  function toggleDay(day) {
    setScheduleDays(prev =>
      prev.includes(day) ? prev.filter(d => d !== day) : [...prev, day]
    )
  }

  function handleSave() {
    const fields = {
      title: title.trim(),
      status,
      due_date: targetDate || null,
      domain_id: domainId ? Number(domainId) : null,
    }
    if (type === "measurable") {
      fields.current_value = Number(currentValue)
      fields.target_value = Number(targetValue)
    }
    if (type === "habit" && habitType === "schedule") {
      fields.schedule_days = JSON.stringify(scheduleDays)
    }
    onSave(goal.ID, fields)
    onClose()
  }

  function handleCancel() {
    onClose()
  }

  function handleDelete() {
    onDelete(goal.ID)
    onClose()
  }

  const typeBadge = (
    <span class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 text-app-muted">
      {type}{habitType ? ` / ${habitType}` : ""}
    </span>
  )


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
            <button
              type="button"
              onClick={onClose}
              class="text-app-muted hover:text-app-ink transition-colors cursor-pointer"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 6 6 18" />
                <path d="m6 6 12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Body */}
        <div class="p-6 flex-1 overflow-y-auto">
          <div class="space-y-4">
            {/* Type badge (read-only) */}
            <div class="flex items-center gap-2">
              {typeBadge}
            </div>

            {/* Status */}
            <div>
              <label class="text-[10px] mono uppercase text-app-muted block mb-1">Status</label>
              <select
                value={status}
                onChange={(e) => setStatus(e.currentTarget.value)}
                class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all text-sm"
              >
                {STATUS_OPTIONS.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>

            {/* Target date */}
            <div>
              <label class="text-[10px] mono uppercase text-app-muted block mb-1">Target date</label>
              <input
                type="date"
                value={targetDate}
                onInput={(e) => setTargetDate(e.currentTarget.value)}
                class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all text-sm"
              />
            </div>

            {/* Domain */}
            <div>
              <label class="text-[10px] mono uppercase text-app-muted block mb-1">Domain</label>
              <select
                value={domainId}
                onChange={(e) => setDomainId(e.currentTarget.value)}
                class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all text-sm"
              >
                <option value="">None</option>
                {(domains || []).map(d => (
                  <option key={d.ID} value={d.ID}>{d.Name || d.name}</option>
                ))}
              </select>
            </div>

            {/* Measurable fields */}
            {type === "measurable" && (
              <div class="flex gap-4">
                <div class="flex-1">
                  <label class="text-[10px] mono uppercase text-app-muted block mb-1">Current value</label>
                  <input
                    type="number"
                    value={currentValue}
                    onInput={(e) => setCurrentValue(e.currentTarget.value)}
                    class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all text-sm"
                  />
                </div>
                <div class="flex-1">
                  <label class="text-[10px] mono uppercase text-app-muted block mb-1">Target value</label>
                  <input
                    type="number"
                    value={targetValue}
                    onInput={(e) => setTargetValue(e.currentTarget.value)}
                    class="w-full bg-black/5 border border-app-border rounded-xl px-3 py-2 outline-none focus:border-app-ink/30 transition-all text-sm"
                  />
                </div>
              </div>
            )}

            {/* Habit streak (display only) */}
            {type === "habit" && habitType === "streak" && (
              <div>
                <label class="text-[10px] mono uppercase text-app-muted block mb-1">Streak</label>
                <p class="text-sm mono">{goal.CompletedTasks || 0} days</p>
              </div>
            )}

            {/* Habit schedule days */}
            {type === "habit" && habitType === "schedule" && (
              <div>
                <label class="text-[10px] mono uppercase text-app-muted block mb-1">Schedule</label>
                <div class="flex items-center gap-2">
                  {DAYS.map((day, i) => (
                    <button
                      key={day}
                      type="button"
                      onClick={() => toggleDay(day)}
                      class={`w-8 h-8 rounded-full mono text-[10px] font-bold transition-all cursor-pointer ${
                        scheduleDays.includes(day)
                          ? "bg-amber-500 text-white"
                          : "bg-black/5 text-app-muted hover:bg-black/10"
                      }`}
                    >
                      {DAY_LABELS[i]}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div class="p-6 pt-0 flex items-center justify-between">
          <div>
            {onDelete && !confirmingDelete && (
              <button
                type="button"
                onClick={() => setConfirmingDelete(true)}
                class="text-[10px] mono uppercase font-bold text-red-400 hover:text-red-600 transition-colors cursor-pointer"
              >
                Delete
              </button>
            )}
            {confirmingDelete && (
              <div class="flex items-center gap-3">
                <span class="text-[10px] text-app-muted">
                  {openTaskCount > 0
                    ? `Delete "${goal.Title}"? This will also delete ${openTaskCount} open task${openTaskCount === 1 ? "" : "s"}.`
                    : `Delete "${goal.Title}"? This cannot be undone.`
                  }
                </span>
                <button
                  type="button"
                  onClick={handleDelete}
                  class="text-[10px] mono uppercase font-bold text-red-400 hover:text-red-600 transition-colors cursor-pointer"
                >
                  Confirm
                </button>
                <button
                  type="button"
                  onClick={() => setConfirmingDelete(false)}
                  class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer"
                >
                  Cancel
                </button>
              </div>
            )}
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              onClick={handleCancel}
              class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={!title.trim()}
              class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              Save
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default GoalModal
