import { useState } from "preact/hooks"
import { cn } from "@/lib/utils.js"
import { dispatchCoderTask } from "@/api/queries.js"
import { typeColor } from "@/constants/colors.js"

const TYPE_OPTIONS = ["task", "note", "goal"]
const PRIORITY_OPTIONS = ["high", "normal", "low"]
const GOAL_TYPE_OPTIONS = ["binary", "measurable", "habit"]
const HABIT_TYPE_OPTIONS = ["streak", "schedule"]
const DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"]

const inputClass =
  "py-2.5 px-3 bg-black/5 border border-app-border rounded-xl text-sm w-full outline-none focus:border-app-ink/30 transition-all"
const selectClass =
  "py-2.5 px-3 bg-black/5 border border-app-border rounded-xl text-sm w-full outline-none focus:border-app-ink/30 transition-all appearance-none cursor-pointer"

function formatTime(ts) {
  if (!ts) return ""
  const d = new Date(ts)
  const now = new Date()
  const diff = now - d
  if (diff < 60000) return "just now"
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" })
}

function truncate(str, len) {
  if (!str) return ""
  return str.length > len ? str.slice(0, len) + "\u2026" : str
}

function inferInitialType(item) {
  const t = item.Type
  if (t === "goal" || t === "task" || t === "note") return t
  return "task"
}

function inferInitialDomain(item) {
  const d = item.DomainID
  if (d && typeof d === "object" && d.Valid) return String(d.Int64)
  if (typeof d === "number" && d > 0) return String(d)
  return ""
}

function TypeBadge({ type }) {
  const color = typeColor(type)
  return (
    <span
      class="text-[9px] mono uppercase px-2 py-0.5 rounded bg-black/5 font-bold shrink-0"
      style={{ color }}
    >
      {type || "item"}
    </span>
  )
}

function PillSelect({ options, value, onChange, labels }) {
  return (
    <div class="flex gap-1.5">
      {options.map((opt, i) => (
        <button
          key={opt}
          type="button"
          onClick={() => onChange(opt)}
          class={cn(
            "text-[10px] mono uppercase font-bold py-1.5 px-3 rounded-lg border-0 cursor-pointer transition-all capitalize",
            value === opt
              ? "bg-amber-500/10 text-amber-600"
              : "bg-black/5 text-app-muted hover:bg-black/10",
          )}
        >
          {labels ? labels[i] : opt}
        </button>
      ))}
    </div>
  )
}

const InboxItem = ({ item, domains, goals, onProcess, onArchive }) => {
  const [expanded, setExpanded] = useState(false)
  const [type, setType] = useState(() => inferInitialType(item))
  const [title, setTitle] = useState(() => item.Title || truncate(item.Raw, 60))
  const [domainId, setDomainId] = useState(() => inferInitialDomain(item))
  const [dueDate, setDueDate] = useState("")
  const [priority, setPriority] = useState("normal")
  const [content, setContent] = useState("")
  const [tags, setTags] = useState("")
  const [context, setContext] = useState("")
  const [goalId, setGoalId] = useState("")
  const [recurring, setRecurring] = useState("")

  const [goalType, setGoalType] = useState("binary")
  const [targetValue, setTargetValue] = useState("")
  const [currentValue, setCurrentValue] = useState("")
  const [habitType, setHabitType] = useState("streak")
  const [scheduleDays, setScheduleDays] = useState([])

  const [submitting, setSubmitting] = useState(false)
  const [dispatching, setDispatching] = useState(false)
  const [dispatchChain, setDispatchChain] = useState("single")
  const [dispatchSubmitting, setDispatchSubmitting] = useState(false)
  const [dispatchDone, setDispatchDone] = useState(false)

  function toggleDay(day) {
    setScheduleDays((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day],
    )
  }

  function handleProcess(e) {
    e.preventDefault()
    if (submitting) return
    setSubmitting(true)

    const payload = {
      type,
      title: title.trim(),
      ...(domainId && { domain_id: Number(domainId) }),
    }

    if (type === "task") {
      if (dueDate) payload.due_date = dueDate
      if (priority !== "normal") payload.priority = priority
      if (goalId) payload.goal_id = goalId
      if (recurring) payload.recurring = recurring
      if (context.trim()) payload.context = context.trim()
    }

    if (type === "note") {
      if (content.trim()) payload.content = content.trim()
      if (tags.trim()) payload.tags = tags.trim()
      if (goalId) payload.goal_id = goalId
    }

    if (type === "goal") {
      payload.goal_type = goalType
      if (context.trim()) payload.context = context.trim()
      if (dueDate) payload.due_date = dueDate
      if (goalType === "measurable") {
        if (targetValue) payload.target_value = Number(targetValue)
        if (currentValue) payload.current_value = Number(currentValue)
      }
      if (goalType === "habit") {
        payload.habit_type = habitType
        if (habitType === "schedule" && scheduleDays.length > 0) {
          payload.schedule_days = JSON.stringify(scheduleDays)
        }
      }
    }

    onProcess(item.ID, payload).finally(() => setSubmitting(false))
  }

  function handleArchive(e) {
    if (e) e.stopPropagation()
    if (submitting) return
    setSubmitting(true)
    onArchive(item.ID).finally(() => setSubmitting(false))
  }

  async function handleDispatch(e) {
    e.stopPropagation()
    if (dispatchSubmitting) return
    setDispatchSubmitting(true)
    const task = [item.Title, item.Summary].filter(Boolean).join(" — ")
    try {
      await dispatchCoderTask({ task, chain: dispatchChain })
      setDispatchDone(true)
      setDispatching(false)
    } finally {
      setDispatchSubmitting(false)
    }
  }

  const createLabel =
    type === "goal" ? "Create Goal" : type === "note" ? "Create Note" : "Create Task"

  if (!expanded) {
    return (
      <div class="p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow group hover:border-app-ink/10 transition-all flex flex-col gap-4 mb-4">
        <div class="flex justify-between items-start gap-3">
          <div class="flex items-center gap-3 min-w-0">
            <TypeBadge type={inferInitialType(item)} />
            <h4 class="text-lg font-bold leading-tight truncate">
              {item.Title || truncate(item.Raw, 80)}
            </h4>
          </div>
          <span class="text-[10px] mono opacity-30 shrink-0">
            {formatTime(item.CreatedAt)}
          </span>
        </div>

        {item.Summary && (
          <p class="text-sm text-app-muted opacity-60 leading-snug">
            {item.Summary}
          </p>
        )}

        <div class="flex gap-2 flex-wrap">
          <button
            type="button"
            onClick={() => setExpanded(true)}
            class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all"
          >
            Process
          </button>
          <button
            type="button"
            onClick={handleArchive}
            disabled={submitting}
            class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all disabled:opacity-40"
          >
            Archive
          </button>
          {dispatchDone ? (
            <span class="px-4 py-2 rounded-xl text-[10px] mono uppercase font-bold text-violet-500">
              Dispatched ✓
            </span>
          ) : dispatching ? (
            <div class="flex items-center gap-2">
              <select
                value={dispatchChain}
                onChange={(e) => setDispatchChain(e.currentTarget.value)}
                class="py-1.5 px-2 bg-black/5 border border-app-border rounded-lg text-[10px] mono uppercase outline-none cursor-pointer"
              >
                <option value="single">Single</option>
                <option value="fix">Fix</option>
                <option value="implement">Implement</option>
                <option value="spec">Spec</option>
              </select>
              <button
                type="button"
                onClick={handleDispatch}
                disabled={dispatchSubmitting}
                class="px-4 py-2 rounded-xl bg-violet-500/10 text-violet-600 text-[10px] mono uppercase font-bold hover:bg-violet-500 hover:text-white transition-all disabled:opacity-40"
              >
                {dispatchSubmitting ? "Sending…" : "Go"}
              </button>
              <button
                type="button"
                onClick={() => setDispatching(false)}
                class="px-3 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all"
              >
                ✕
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); setDispatching(true) }}
              class="px-4 py-2 rounded-xl bg-violet-500/10 text-violet-600 text-[10px] mono uppercase font-bold hover:bg-violet-500 hover:text-white transition-all"
            >
              → Agent
            </button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div class="p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow mb-4">
      <div class="flex items-start justify-between gap-3 mb-5">
        <div class="flex items-center gap-3 min-w-0">
          <TypeBadge type={inferInitialType(item)} />
          <span class="text-[10px] mono opacity-30">{formatTime(item.CreatedAt)}</span>
        </div>
        <button
          type="button"
          onClick={() => setExpanded(false)}
          class="text-[10px] mono uppercase text-app-muted hover:text-app-ink transition-all cursor-pointer shrink-0"
        >
          Collapse
        </button>
      </div>

      <div class="text-sm leading-relaxed mb-5 p-4 bg-black/5 rounded-2xl border border-app-border opacity-70">
        {item.Raw}
      </div>

      <form onSubmit={handleProcess} class="flex flex-col gap-3">
        <PillSelect options={TYPE_OPTIONS} value={type} onChange={setType} />

        <input
          type="text"
          value={title}
          onInput={(e) => setTitle(e.currentTarget.value)}
          placeholder="Title"
          class={inputClass}
        />

        <select
          value={domainId}
          onChange={(e) => setDomainId(e.currentTarget.value)}
          class={selectClass}
        >
          <option value="">No domain</option>
          {(domains || []).map((d) => (
            <option key={d.ID} value={d.ID}>
              {d.Name}
            </option>
          ))}
        </select>

        {type === "task" && (
          <>
            <div class="flex gap-3">
              <input
                type="date"
                value={dueDate}
                onInput={(e) => setDueDate(e.currentTarget.value)}
                class={cn(inputClass, "flex-1")}
              />
              <select
                value={priority}
                onChange={(e) => setPriority(e.currentTarget.value)}
                class={cn(selectClass, "flex-1")}
              >
                {PRIORITY_OPTIONS.map((p) => (
                  <option key={p} value={p}>
                    {p.charAt(0).toUpperCase() + p.slice(1)}
                  </option>
                ))}
              </select>
            </div>
            <select
              value={goalId}
              onChange={(e) => setGoalId(e.currentTarget.value)}
              class={selectClass}
            >
              <option value="">No goal alignment</option>
              {(goals || []).map((g) => (
                <option key={g.ID} value={g.ID}>
                  {g.Title}
                </option>
              ))}
            </select>
            <select
              value={recurring}
              onChange={(e) => setRecurring(e.currentTarget.value)}
              class={selectClass}
            >
              <option value="">Not recurring</option>
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
            </select>
            <input
              type="text"
              value={context}
              onInput={(e) => setContext(e.currentTarget.value)}
              placeholder="Context (helps the agent estimate time and suggest scheduling)"
              class={inputClass}
            />
          </>
        )}

        {type === "note" && (
          <>
            <input
              type="text"
              value={content}
              onInput={(e) => setContent(e.currentTarget.value)}
              placeholder="Content"
              class={inputClass}
            />
            <input
              type="text"
              value={tags}
              onInput={(e) => setTags(e.currentTarget.value)}
              placeholder="Tags (comma-separated)"
              class={inputClass}
            />
            <select
              value={goalId}
              onChange={(e) => setGoalId(e.currentTarget.value)}
              class={selectClass}
            >
              <option value="">No goal alignment</option>
              {(goals || []).map((g) => (
                <option key={g.ID} value={g.ID}>
                  {g.Title}
                </option>
              ))}
            </select>
          </>
        )}

        {type === "goal" && (
          <>
            <div>
              <label class="text-[9px] mono uppercase tracking-widest opacity-40 mb-2 block">
                Goal Type
              </label>
              <PillSelect
                options={GOAL_TYPE_OPTIONS}
                value={goalType}
                onChange={setGoalType}
              />
            </div>

            {goalType === "measurable" && (
              <div class="flex gap-3">
                <div class="flex-1">
                  <label class="text-[9px] mono uppercase tracking-widest opacity-40 mb-1 block">
                    Target
                  </label>
                  <input
                    type="number"
                    value={targetValue}
                    onInput={(e) => setTargetValue(e.currentTarget.value)}
                    placeholder="e.g. 185"
                    class={inputClass}
                  />
                </div>
                <div class="flex-1">
                  <label class="text-[9px] mono uppercase tracking-widest opacity-40 mb-1 block">
                    Current
                  </label>
                  <input
                    type="number"
                    value={currentValue}
                    onInput={(e) => setCurrentValue(e.currentTarget.value)}
                    placeholder="e.g. 235"
                    class={inputClass}
                  />
                </div>
              </div>
            )}

            {goalType === "habit" && (
              <>
                <div>
                  <label class="text-[9px] mono uppercase tracking-widest opacity-40 mb-2 block">
                    Habit Type
                  </label>
                  <PillSelect
                    options={HABIT_TYPE_OPTIONS}
                    value={habitType}
                    onChange={setHabitType}
                  />
                </div>
                {habitType === "schedule" && (
                  <div>
                    <label class="text-[9px] mono uppercase tracking-widest opacity-40 mb-2 block">
                      Schedule
                    </label>
                    <div class="flex gap-1.5">
                      {DAYS.map((day, i) => (
                        <button
                          key={day}
                          type="button"
                          onClick={() => toggleDay(day)}
                          class={cn(
                            "w-10 h-10 rounded-xl text-[10px] mono font-bold cursor-pointer transition-all",
                            scheduleDays.includes(day)
                              ? "bg-amber-500/10 text-amber-600"
                              : "bg-black/5 text-app-muted hover:bg-black/10",
                          )}
                        >
                          {DAY_LABELS[i]}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}

            <input
              type="text"
              value={context}
              onInput={(e) => setContext(e.currentTarget.value)}
              placeholder="Context / description"
              class={inputClass}
            />
            <input
              type="date"
              value={dueDate}
              onInput={(e) => setDueDate(e.currentTarget.value)}
              class={inputClass}
            />
          </>
        )}

        <div class="flex items-center justify-end gap-2 mt-2">
          <button
            type="button"
            onClick={handleArchive}
            disabled={submitting}
            class="px-4 py-2 rounded-xl bg-black/5 text-app-muted text-[10px] mono uppercase font-bold hover:bg-black/10 transition-all disabled:opacity-40"
          >
            Archive
          </button>
          <button
            type="submit"
            disabled={submitting || !title.trim()}
            class="px-4 py-2 rounded-xl bg-amber-500/10 text-amber-600 text-[10px] mono uppercase font-bold hover:bg-amber-500 hover:text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? "Creating\u2026" : createLabel}
          </button>
        </div>
      </form>
    </div>
  )
}

export default InboxItem
