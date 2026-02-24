import { useState } from "preact/hooks"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { setTaskFocus, clearTaskFocus } from "@/api/queries.js"
import EmptyState from "@/components/ui/EmptyState.jsx"
import TaskModal from "@/components/notebooks/TaskModal.jsx"

const DOMAIN_COLORS = {
  "Work/Business": "#5B8DB8",
  "Personal Projects": "#8B6BB1",
  "Homelife": "#C47D3A",
  "Personal Development": "#5A9E6F",
  "Relationships": "#C4617A",
  "Finances": "#3D9E9E",
  "Hobbies": "#C4663A",
}

function domainColor(name) {
  if (!name) return "#9C8E7A"
  return DOMAIN_COLORS[name] || "#9C8E7A"
}

function parseMicroSteps(task) {
  if (!task.MicroSteps?.Valid || !task.MicroSteps.String) return []
  try {
    return JSON.parse(task.MicroSteps.String)
  } catch {
    return []
  }
}

const CircleIcon = ({ size = 24 }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="10" />
  </svg>
)

const CircleCheckIcon = ({ size = 16 }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-emerald-500" aria-hidden="true">
    <circle cx="12" cy="12" r="10" /><path d="m9 12 2 2 4-4" />
  </svg>
)

const TargetIcon = ({ size = 14 }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="10" /><circle cx="12" cy="12" r="6" /><circle cx="12" cy="12" r="2" />
  </svg>
)

function TheOneTask({ task, onComplete, onClearFocus, isCompleting, domains, goals, onSave, onDelete }) {
  const steps = parseMicroSteps(task)
  const color = domainColor(task.DomainName)
  const isPinned = task.IsFocus === 1
  const [modalOpen, setModalOpen] = useState(false)
  const [modalEditMode, setModalEditMode] = useState(false)

  return (
    <div class={`relative p-6 bg-[var(--color-card-bg)] rounded-3xl border border-app-border card-shadow overflow-hidden transition-all duration-300 group ${isCompleting ? "opacity-0 translate-y-2 scale-95" : ""}`}>
      <div class="absolute left-0 top-0 bottom-0 w-1.5" style={{ backgroundColor: color }} />

      <div class="flex gap-4 pl-2">
        <div class="flex-1 space-y-4">
          <div class="flex items-start gap-4">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onComplete(task.ID) }}
              class="mt-1 text-app-muted hover:text-app-ink transition-colors"
            >
              <CircleIcon size={24} />
            </button>
            <div class="flex-1 cursor-pointer" onClick={() => { setModalEditMode(false); setModalOpen(true) }}>
              <div class="flex items-center justify-between gap-2">
                <p class="text-[10px] mono uppercase font-bold mb-1" style={{ color }}>The One</p>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); setModalEditMode(true); setModalOpen(true) }}
                    class="opacity-0 group-hover:opacity-100 text-[9px] mono uppercase opacity-30 hover:opacity-60 transition-opacity cursor-pointer"
                    title="Edit task"
                  >
                    edit
                  </button>
                  {isPinned && (
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); onClearFocus() }}
                      class="text-[9px] mono uppercase opacity-30 hover:opacity-60 transition-opacity cursor-pointer"
                      title="Clear focus"
                    >
                      clear
                    </button>
                  )}
                </div>
              </div>
              <h4 class="text-xl font-bold leading-tight">{task.Title}</h4>
              {task.DomainName && (
                <p class="text-[10px] mono uppercase font-bold mt-1 opacity-40">{task.DomainName}</p>
              )}
            </div>
          </div>

          {steps.length > 0 && (
            <div class="pl-10 space-y-3">
              <p class="text-[10px] mono uppercase opacity-40 font-bold">Steps</p>
              {steps.map((step, i) => {
                const isDone = typeof step === "object" ? step.done : false
                const label = typeof step === "object" ? (step.title || step.label || step.text) : step
                return (
                  <div key={i} class="flex items-center gap-3">
                    <button type="button" class={`transition-colors ${isDone ? "text-emerald-500" : "text-app-muted hover:text-app-ink"}`}>
                      {isDone ? <CircleCheckIcon size={16} /> : <CircleIcon size={16} />}
                    </button>
                    <span class={`text-sm font-medium ${isDone ? "line-through opacity-30" : "opacity-70"}`}>
                      {label}
                    </span>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {modalOpen && (
        <TaskModal
          task={task}
          domains={domains}
          goals={goals}
          initialEditMode={modalEditMode}
          onClose={() => setModalOpen(false)}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </div>
  )
}

function CompactTaskItem({ task, onComplete, onSetFocus, isCompleting, domains, goals, onSave, onDelete }) {
  const color = domainColor(task.DomainName)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalEditMode, setModalEditMode] = useState(false)

  return (
    <div class={`flex items-center gap-3 p-3 bg-[var(--color-card-bg)] rounded-xl border border-app-border hover:border-app-ink/10 transition-all duration-300 group ${isCompleting ? "opacity-0 translate-y-2 scale-95" : ""}`}>
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onComplete(task.ID) }}
        class="text-app-muted hover:text-app-ink transition-colors"
      >
        <CircleIcon size={18} />
      </button>
      <span
        class="text-sm font-medium flex-1 cursor-pointer"
        onClick={() => { setModalEditMode(false); setModalOpen(true) }}
      >
        {task.Title}
      </span>
      {task.DomainName && (
        <span class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 opacity-60" style={{ color }}>
          {task.DomainName}
        </span>
      )}
      <button
        type="button"
        onClick={() => { setModalEditMode(true); setModalOpen(true) }}
        class="opacity-0 group-hover:opacity-100 text-[9px] mono uppercase text-app-muted hover:text-app-ink transition-opacity cursor-pointer"
        title="Edit task"
      >
        edit
      </button>
      <button
        type="button"
        onClick={() => onSetFocus(task.ID)}
        class="opacity-0 group-hover:opacity-30 hover:!opacity-70 text-app-muted transition-opacity cursor-pointer"
        title="Set as focus"
      >
        <TargetIcon size={14} />
      </button>

      {modalOpen && (
        <TaskModal
          task={task}
          domains={domains}
          goals={goals}
          initialEditMode={modalEditMode}
          onClose={() => setModalOpen(false)}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </div>
  )
}

const TABS = [
  { key: "today", label: "Today" },
  { key: "tomorrow", label: "Tomorrow" },
  { key: "this week", label: "This Week" },
]

const TaskList = ({ tasks = [], overdueTasks = [], tomorrowTasks = [], weekTasks = [], onComplete, domains = [], goals = [], onSaveTask, onDeleteTask }) => {
  const [tab, setTab] = useState("today")
  const [completing, setCompleting] = useState(() => new Set())
  const queryClient = useQueryClient()

  const handleComplete = (id) => {
    setCompleting(prev => new Set(prev).add(id))
    onComplete(id)
    setTimeout(() => setCompleting(prev => {
      const next = new Set(prev)
      next.delete(id)
      return next
    }), 400)
  }

  const focusMutation = useMutation({
    mutationFn: setTaskFocus,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
  })

  const clearFocusMutation = useMutation({
    mutationFn: clearTaskFocus,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
  })

  const seen = new Set()
  const todayCombined = [
    ...tasks.filter(t => t.IsFocus === 1),
    ...overdueTasks.map(t => ({ ...t, _overdue: true })).filter(t => t.IsFocus !== 1),
    ...tasks.filter(t => t.IsFocus !== 1),
  ].filter(t => {
    if (seen.has(t.ID)) return false
    seen.add(t.ID)
    return true
  })

  const activeTasks = tab === "today" ? todayCombined
    : tab === "tomorrow" ? (tomorrowTasks || [])
    : (weekTasks || [])

  const theOne = tab === "today" && activeTasks.length > 0 ? activeTasks[0] : null
  const alsoToday = tab === "today" && activeTasks.length > 1 ? activeTasks.slice(1) : []
  const otherTasks = tab !== "today" ? activeTasks : []

  return (
    <div class="space-y-6">
      <div class="flex items-center justify-between border-b border-app-border pb-2">
        <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">Today's Focus</h3>
        <div class="flex gap-1">
          {TABS.map(t => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              class={`px-2.5 py-1 text-[10px] mono uppercase tracking-wide rounded-full transition-colors cursor-pointer ${
                tab === t.key
                  ? "bg-app-ink/8 text-app-ink font-semibold"
                  : "text-app-muted hover:text-app-ink hover:bg-app-ink/4"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {!activeTasks.length ? (
        <EmptyState message={
          tab === "today" ? "Nothing due today. Check your week or add something."
          : tab === "tomorrow" ? "Nothing due tomorrow."
          : "Nothing due this week."
        } />
      ) : tab === "today" ? (
        <div class="space-y-6">
          {theOne && (
            <TheOneTask
              task={theOne}
              onComplete={handleComplete}
              onClearFocus={() => clearFocusMutation.mutate()}
              isCompleting={completing.has(theOne.ID)}
              domains={domains}
              goals={goals}
              onSave={onSaveTask}
              onDelete={onDeleteTask}
            />
          )}

          {alsoToday.length > 0 && (
            <div class="space-y-3">
              <h3 class="text-[10px] mono uppercase tracking-widest opacity-40">Also Today</h3>
              {alsoToday.map(task => (
                <CompactTaskItem
                  key={task.ID}
                  task={task}
                  onComplete={handleComplete}
                  onSetFocus={(id) => focusMutation.mutate(id)}
                  isCompleting={completing.has(task.ID)}
                  domains={domains}
                  goals={goals}
                  onSave={onSaveTask}
                  onDelete={onDeleteTask}
                />
              ))}
            </div>
          )}
        </div>
      ) : (
        <div class="space-y-1">
          {otherTasks.map(task => (
            <CompactTaskItem
              key={task.ID}
              task={task}
              onComplete={handleComplete}
              onSetFocus={(id) => focusMutation.mutate(id)}
              isCompleting={completing.has(task.ID)}
              domains={domains}
              goals={goals}
              onSave={onSaveTask}
              onDelete={onDeleteTask}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default TaskList
