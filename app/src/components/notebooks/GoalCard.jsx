import { useState } from "preact/hooks"
import GoalModal from "./GoalModal.jsx"

const DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"]

function parseScheduleDays(goal) {
  try { return JSON.parse(goal.ScheduleDays?.String || "[]") } catch { return [] }
}

const TargetIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <circle cx="12" cy="12" r="6" />
    <circle cx="12" cy="12" r="2" />
  </svg>
)

const FlameIcon = ({ color }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill={color} stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z" />
  </svg>
)

const ProgressBar = ({ pct, color }) => (
  <div class="h-2 w-full bg-black/5 rounded-full overflow-hidden">
    <div class="h-full rounded-full transition-[width] duration-700 ease-out" style={{ backgroundColor: color, width: `${pct}%` }} />
  </div>
)

const ScheduleViz = ({ goal, color }) => {
  const scheduled = parseScheduleDays(goal).map(d => d.toLowerCase())
  const today = new Date().toLocaleDateString("en-US", { weekday: "long" }).toLowerCase()
  const todayIdx = DAYS.indexOf(today)

  return (
    <div class="flex items-center gap-1">
      {DAYS.map((day, i) => {
        const isScheduled = scheduled.includes(day)
        const isDone = isScheduled && i <= todayIdx
        return (
          <div key={day} class="flex flex-col items-center gap-1">
            <div
              class="w-5 h-5 rounded-full transition-all"
              style={{
                backgroundColor: isDone ? color : "transparent",
                border: isScheduled ? `2px solid ${color}` : "2px solid rgba(0,0,0,0.08)",
                opacity: isScheduled ? 1 : 0.3,
              }}
            />
            <span class="mono text-[8px] opacity-30">{DAY_LABELS[i]}</span>
          </div>
        )
      })}
    </div>
  )
}

const GoalCard = ({ goal, domain, color, onSave, onDelete, domains, tasks }) => {
  const [modalOpen, setModalOpen] = useState(false)
  const [modalEditMode, setModalEditMode] = useState(false)
  const resolvedColor = domain?.color || domain?.Color?.String || color || "#9C8E7A"
  const domainName = domain?.name || domain?.Name
  const pct = Math.min(Math.round((goal.Progress || 0) * 100), 100)
  const type = goal.Type || "binary"
  const habitType = goal.HabitType?.String

  return (
    <>
    <div
      class="p-4 bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow group hover:border-app-ink/10 transition-all cursor-pointer"
      onClick={() => { setModalEditMode(false); setModalOpen(true) }}
    >
      <div class="flex justify-between items-start mb-3">
        <div>
          {domainName && (
            <span
              class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 opacity-60 mb-1 inline-block"
              style={{ color: resolvedColor }}
            >
              {domainName}
            </span>
          )}
          <h4 class="font-semibold text-sm leading-tight">{goal.Title}</h4>
        </div>
        <div class="flex items-center gap-2 shrink-0 ml-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); setModalEditMode(true); setModalOpen(true) }}
            class="text-[10px] mono uppercase font-bold text-app-muted hover:text-app-ink transition-colors cursor-pointer opacity-0 group-hover:opacity-100"
          >
            Edit
          </button>
          <span class="opacity-20"><TargetIcon /></span>
        </div>
      </div>

      {type === "measurable" ? (
        <div class="space-y-2">
          <div class="flex items-baseline gap-1">
            <span class="text-xl font-bold mono">{goal.CurrentValue?.Float64 ?? 0}</span>
            <span class="text-xs text-app-muted mono">/ {goal.TargetValue?.Float64 ?? 0}</span>
          </div>
          <ProgressBar pct={pct} color={resolvedColor} />
        </div>
      ) : type === "habit" && habitType === "streak" ? (
        <div class="flex items-center gap-2">
          <FlameIcon color={resolvedColor} />
          <span class="mono text-lg font-semibold" style={{ color: resolvedColor }}>
            {goal.CompletedTasks || 0}
          </span>
          <span class="text-xs text-app-muted mono">day streak</span>
        </div>
      ) : type === "habit" && habitType === "schedule" ? (
        <ScheduleViz goal={goal} color={resolvedColor} />
      ) : (
        <div class="space-y-2">
          {goal.TotalTasks > 0 && (
            <p class="text-[10px] mono text-app-muted">{goal.CompletedTasks}/{goal.TotalTasks} tasks</p>
          )}
          <ProgressBar pct={pct} color={resolvedColor} />
        </div>
      )}
    </div>
    {modalOpen && (
      <GoalModal
        goal={goal}
        domains={domains}
        tasks={tasks}
        initialEditMode={modalEditMode}
        onClose={() => setModalOpen(false)}
        onSave={onSave}
        onDelete={onDelete}
      />
    )}
    </>
  )
}

export default GoalCard
