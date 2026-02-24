import { useQuery } from "@tanstack/react-query"
import { domainHealthQuery } from "@/api/queries.js"
import { domainColor, driftClass } from "@/constants/colors.js"

const BookIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
    <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
  </svg>
)

const ChevronRight = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="m9 18 6-6-6-6" />
  </svg>
)

const ProgressBar = ({ progress, color }) => (
  <div class="space-y-1.5">
    <div class="flex justify-between text-[10px] mono uppercase opacity-50">
      <span>Health</span>
      <span>{progress}%</span>
    </div>
    <div class="h-2 w-full bg-black/5 rounded-full overflow-hidden">
      <div class="h-full rounded-full transition-[width] duration-700 ease-out" style={{ backgroundColor: color, width: `${progress}%` }} />
    </div>
  </div>
)

const DomainCard = ({ domain, onOpen }) => {
  const color = domainColor(domain)
  const drift = (domain.State || "active").toLowerCase()
  const driftCls = driftClass(drift)
  const health = Math.round((domain.HealthScore || 0) * 100)

  return (
    <div
      onClick={() => onOpen(domain.ID)}
      class="p-6 bg-white rounded-3xl border border-app-border border-t-2 card-shadow flex flex-col gap-6 cursor-pointer group transition-transform duration-200 hover:-translate-y-1"
      style={{ borderTopColor: color }}
    >
      <div class="flex justify-between items-start">
        <div
          class="w-12 h-12 rounded-2xl flex items-center justify-center text-white shadow-lg"
          style={{ backgroundColor: color }}
        >
          <BookIcon />
        </div>
        <div class="text-right">
          <h3 class="font-bold text-lg">{domain.Name}</h3>
          <span class={`text-[9px] mono uppercase px-1.5 py-0.5 rounded-full ${driftCls}`}>
            {drift}
          </span>
        </div>
      </div>

      <div class="space-y-4">
        <ProgressBar progress={health} color={color} />
        <div class="flex gap-4 pt-2">
          <div class="flex flex-col">
            <span class="text-lg font-bold mono">{domain.GoalCount || 0}</span>
            <span class="text-[9px] mono uppercase opacity-40">Goals</span>
          </div>
          <div class="flex flex-col">
            <span class="text-lg font-bold mono">{domain.TaskCount || 0}</span>
            <span class="text-[9px] mono uppercase opacity-40">Tasks</span>
          </div>
          <div class="flex flex-col">
            <span class="text-lg font-bold mono">{domain.NoteCount || 0}</span>
            <span class="text-[9px] mono uppercase opacity-40">Notes</span>
          </div>
        </div>
      </div>

      <div class="mt-auto flex items-center justify-between text-app-muted group-hover:text-app-ink transition-colors">
        <span class="text-xs font-bold">Open Notebook</span>
        <ChevronRight />
      </div>
    </div>
  )
}

const Notebooks = ({ onOpenNotebook }) => {
  const { data: domains, isLoading } = useQuery(domainHealthQuery)

  const active = (domains || []).filter(d => d.Status !== "archived")

  if (isLoading) {
    return (
      <div class="space-y-8">
        <header>
          <h2 class="text-3xl font-serif italic font-semibold">Notebooks</h2>
          <p class="text-app-muted text-sm mt-1">Deep dives into your life domains.</p>
        </header>
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {[1, 2, 3].map(i => (
            <div key={i} class="h-64 bg-white rounded-3xl border border-app-border animate-pulse" />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div class="space-y-8">
      <header>
        <h2 class="text-3xl font-serif italic font-semibold">Notebooks</h2>
        <p class="text-app-muted text-sm mt-1">Deep dives into your life domains.</p>
      </header>

      {active.length === 0 ? (
        <div class="py-20 text-center opacity-20">
          <p class="font-serif italic text-lg">No domains yet.</p>
        </div>
      ) : (
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {active.map(d => (
            <DomainCard key={d.ID} domain={d} onOpen={onOpenNotebook} />
          ))}
        </div>
      )}
    </div>
  )
}

export default Notebooks
