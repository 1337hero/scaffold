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
  return DOMAIN_COLORS[name] || DOMAIN_COLORS[name.toLowerCase()] || "#9C8E7A"
}

const DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"]

function parseScheduleDays(goal) {
  try {
    return JSON.parse(goal.ScheduleDays?.String || "[]")
  } catch {
    return []
  }
}

const FlameIcon = ({ color }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill={color} stroke={color} stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z" />
  </svg>
)

const ProgressBar = ({ value, color }) => (
  <div class="h-2 w-full bg-black/5 rounded-full overflow-hidden">
    <div
      class="h-full rounded-full transition-[width] duration-700 ease-out"
      style={{ backgroundColor: color, width: `${Math.round(value * 100)}%` }}
    />
  </div>
)

const BinaryViz = ({ goal, color }) => (
  <ProgressBar value={goal.Progress || 0} color={color} />
)

const MeasurableViz = ({ goal, color }) => (
  <div>
    {goal.TargetValue?.Valid && (
      <div class="mono text-xs mb-1" style={{ color }}>
        {Math.round(goal.CurrentValue?.Float64 || 0)} / {Math.round(goal.TargetValue.Float64)}
      </div>
    )}
    <ProgressBar value={goal.Progress || 0} color={color} />
  </div>
)

const StreakViz = ({ goal, color }) => (
  <div class="flex items-center gap-2">
    <FlameIcon color={color} />
    <span class="mono text-lg font-semibold" style={{ color }}>
      {goal.CompletedTasks || 0}
    </span>
  </div>
)

const ScheduleViz = ({ goal, color }) => {
  const scheduled = parseScheduleDays(goal).map(d => d.toLowerCase())
  const today = new Date().toLocaleDateString("en-US", { weekday: "long" }).toLowerCase()
  const todayIdx = DAYS.indexOf(today)

  return (
    <div class="flex items-center gap-1.5">
      {DAYS.map((day, i) => {
        const isScheduled = scheduled.includes(day)
        const isPast = i <= todayIdx
        const isDone = isScheduled && isPast

        return (
          <div key={day} class="flex flex-col items-center gap-1">
            <div
              class="w-4 h-4 rounded-full transition-all"
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

const GoalViz = ({ goal, color }) => {
  const habitType = goal.HabitType?.String || ""
  if (goal.Type === "habit" && habitType === "streak") return <StreakViz goal={goal} color={color} />
  if (goal.Type === "habit" && habitType === "schedule") return <ScheduleViz goal={goal} color={color} />
  if (goal.Type === "measurable") return <MeasurableViz goal={goal} color={color} />
  return <BinaryViz goal={goal} color={color} />
}

const GoalsOverview = ({ goals = [] }) => {
  return (
    <section class="space-y-4">
      <div class="flex items-center gap-2 mb-3">
        <h3 class="text-[10px] mono uppercase tracking-widest opacity-40">Active Goals</h3>
        <div class="h-px flex-1 bg-app-border" />
      </div>

      {goals.length ? (
        <div class="flex gap-4 overflow-x-auto pb-4 scrollbar-hide">
          {goals.map(goal => {
            const color = domainColor(goal.DomainName)
            return (
              <a
                key={goal.ID}
                href={`#/notebooks/${goal.DomainID}`}
                class="block p-4 bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow group hover:border-app-ink/10 transition-all min-w-[240px] no-underline text-inherit cursor-pointer"
              >
                <div class="flex justify-between items-start mb-3">
                  <div>
                    {goal.DomainName && (
                      <span
                        class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 opacity-60 mb-1 inline-block"
                        style={{ color }}
                      >
                        {goal.DomainName}
                      </span>
                    )}
                    <h4 class="font-semibold text-sm leading-tight">{goal.Title}</h4>
                  </div>
                  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="opacity-20" aria-hidden="true">
                    <circle cx="12" cy="12" r="10" /><circle cx="12" cy="12" r="6" /><circle cx="12" cy="12" r="2" />
                  </svg>
                </div>
                <GoalViz goal={goal} color={color} />
              </a>
            )
          })}
        </div>
      ) : (
        <div class="py-8 text-center opacity-30">
          <p class="font-serif italic text-sm">No active goals</p>
        </div>
      )}
    </section>
  )
}

export default GoalsOverview
