import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  inboxQuery,
  domainsQuery,
  goalsQuery,
  processInboxItem,
  archiveInboxCapture,
} from "@/api/queries.js"
import InboxItem from "@/components/inbox/InboxItem.jsx"
import { GROUP_DOTS } from "@/constants/colors.js"

const Inbox = () => {
  const queryClient = useQueryClient()
  const { data: items = [], isLoading } = useQuery(inboxQuery)
  const { data: domains = [] } = useQuery(domainsQuery)
  const { data: goals = [] } = useQuery(goalsQuery())

  const processMutation = useMutation({
    mutationFn: ({ id, data }) => processInboxItem(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["inbox"] })
      queryClient.invalidateQueries({ queryKey: ["inbox-count"] })
      queryClient.invalidateQueries({ queryKey: ["goals"] })
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["notes"] })
      queryClient.invalidateQueries({ queryKey: ["dashboard"] })
    },
  })

  const archiveMutation = useMutation({
    mutationFn: (id) => archiveInboxCapture(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["inbox"] })
      queryClient.invalidateQueries({ queryKey: ["inbox-count"] })
    },
  })

  function handleProcess(id, data) {
    return processMutation.mutateAsync({ id, data })
  }

  function handleArchive(id) {
    return archiveMutation.mutateAsync(id)
  }

  const unprocessed = items.filter((i) => i.Processed === 0)

  const GROUPS = [
    { key: "tasks",  label: "Tasks",  dot: GROUP_DOTS.tasks, types: ["task"] },
    { key: "goals",  label: "Goals",  dot: GROUP_DOTS.goals, types: ["goal"] },
    { key: "notes",  label: "Notes",  dot: GROUP_DOTS.notes, types: ["note", "link", "video", "idea", "article"] },
  ]

  const grouped = GROUPS.map(g => ({
    ...g,
    items: unprocessed.filter(i => g.types.includes(i.Type?.toLowerCase())),
  })).filter(g => g.items.length > 0)

  if (isLoading) {
    return (
      <div style={{ opacity: 1, transform: "none" }}>
        <div class="max-w-4xl mx-auto space-y-12 pb-20">
          <header class="flex items-center justify-between">
            <h2 class="text-4xl font-serif italic font-semibold">Inbox</h2>
          </header>
          <div class="space-y-12">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                class="h-[72px] bg-[var(--color-card-bg)] rounded-3xl border border-app-border animate-pulse"
              />
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div style={{ opacity: 1, transform: "none" }}>
      <div class="max-w-4xl mx-auto space-y-12 pb-20">
        <header class="flex items-center justify-between">
          <h2 class="text-4xl font-serif italic font-semibold">Inbox</h2>
          {unprocessed.length > 0 && (
            <div class="flex gap-2">
              <span class="px-4 py-1.5 rounded-lg bg-white border border-app-border text-[10px] mono uppercase font-bold">
                {unprocessed.length}{" "}
                {unprocessed.length === 1 ? "item" : "items"}
              </span>
            </div>
          )}
        </header>

        <div class="space-y-12">
          {unprocessed.length === 0 ? (
            <div class="flex flex-col items-center justify-center py-16">
              <div class="text-4xl mb-4 opacity-30">&#x2713;</div>
              <p class="text-lg font-semibold mb-1">Inbox zero</p>
              <p class="text-sm text-app-muted">All clear.</p>
            </div>
          ) : (
            <div class="space-y-10">
              {grouped.map(group => (
                <div key={group.key} class="space-y-4">
                  <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: group.dot }} />
                    <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-60">{group.label}</h3>
                    <span class="text-[10px] mono opacity-30">{group.items.length} {group.items.length === 1 ? "item" : "items"}</span>
                  </div>
                  <div class="space-y-3">
                    {group.items.map(item => (
                      <InboxItem
                        key={item.ID}
                        item={item}
                        domains={domains}
                        goals={goals}
                        onProcess={handleProcess}
                        onArchive={handleArchive}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default Inbox
