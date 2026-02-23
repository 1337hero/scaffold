import { useQuery } from '@tanstack/react-query'
import { domainsQuery } from '@/api/queries.js'
import DomainTile from './DomainTile.jsx'

const GROUP_ORDER = [
  { key: "active",     label: "Active",     color: "text-green" },
  { key: "drifting",   label: "Drifting",   color: "text-amber" },
  { key: "overactive", label: "Overactive", color: "text-purple" },
  { key: "cold",       label: "Cold",       color: "text-text-muted" },
  { key: "dump",       label: "The Dump",   color: "text-text-dim" },
  { key: "neglected",  label: "Neglected",  color: "text-red" },
]

function groupDomains(domains) {
  const groups = {}
  for (const g of GROUP_ORDER) groups[g.key] = []

  for (const d of domains) {
    const isDump = d.id === 0 || d.name === "The Dump"
    const key = isDump ? "dump" : (groups[d.drift_state] ? d.drift_state : "drifting")
    groups[key].push(d)
  }

  return GROUP_ORDER
    .filter((g) => groups[g.key].length > 0)
    .map((g) => ({ ...g, domains: groups[g.key] }))
}

const Map = ({ onOpenDomain }) => {
  const { data: domains = [], isLoading } = useQuery(domainsQuery)

  if (isLoading) return (
    <div class="panel-shell">
      <div class="h-6 w-48 animate-pulse bg-surface-card rounded mb-8" />
      {[1, 2].map((g) => (
        <div key={g} class="mb-8">
          <div class="h-3 w-24 animate-pulse bg-surface-card rounded mb-3" />
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[1, 2].map((c) => (
              <div key={c} class="h-20 animate-pulse bg-surface-card rounded" />
            ))}
          </div>
        </div>
      ))}
    </div>
  )

  const groups = groupDomains(domains)

  return (
    <div class="panel-shell">
      <h2 class="panel-title mb-8">Today's Attention Map</h2>

      {groups.length === 0 && (
        <div class="text-text-muted text-[0.95rem] py-12 text-center">
          No domains configured yet.
        </div>
      )}

      {groups.map((group) => (
        <div key={group.key} class="mb-8">
          <div class="flex items-center gap-3 mb-3">
            <span class={`text-[0.72rem] font-semibold uppercase tracking-[0.12em] ${group.color}`}>
              {group.label}
            </span>
            <span class="flex-1 h-px bg-border" />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {group.domains.map((d) => (
              <DomainTile key={d.id} domain={d} onOpenDomain={onOpenDomain} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

export default Map
