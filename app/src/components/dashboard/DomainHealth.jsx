import { domainColor, driftClass } from "@/constants/colors.js"

const ActivityIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2" />
  </svg>
)

const DomainHealth = ({ domains = [] }) => {
  return (
    <section class="space-y-4">
      <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">Domain Health</h3>
      <div class="grid grid-cols-1 gap-3">
        {domains.map(domain => {
          const color = domainColor(domain.Name)
          const drift = (domain.State || "active").toLowerCase()
          const driftCls = driftClass(drift)
          const health = Math.round((domain.HealthScore || 0) * 100)

          return (
            <a
              key={domain.ID}
              href={`#/notebooks/${domain.ID}`}
              class="p-4 bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow flex items-center gap-4 group hover:border-app-ink/10 transition-all cursor-pointer"
            >
              <div
                class="w-10 h-10 rounded-xl flex items-center justify-center text-white shadow-lg"
                style={{ backgroundColor: color }}
              >
                <ActivityIcon />
              </div>
              <div class="flex-1">
                <div class="flex justify-between items-center mb-1">
                  <span class="font-semibold text-sm">{domain.Name}</span>
                  <span class={`text-[9px] mono uppercase px-1.5 py-0.5 rounded-full ${driftCls}`}>
                    {drift}
                  </span>
                </div>
                <div class="h-1.5 w-full bg-black/5 rounded-full overflow-hidden">
                  <div class="h-full rounded-full transition-[width] duration-700 ease-out" style={{ backgroundColor: color, width: `${health}%` }} />
                </div>
              </div>
            </a>
          )
        })}
      </div>
    </section>
  )
}

export default DomainHealth
