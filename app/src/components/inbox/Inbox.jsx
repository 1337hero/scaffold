import { useState } from 'preact/hooks'
import { useQuery } from '@tanstack/react-query'
import { InboxGroup } from './InboxGroup.jsx'
import { inboxQuery } from '@/api/queries.js'

const views = ['By Action', 'By Time', 'By Type']

export function Inbox() {
  const { data: groups = [], isLoading } = useQuery(inboxQuery)
  const [activeView, setActiveView] = useState('By Action')

  if (isLoading) return null

  return (
    <div class="panel-shell">
      <div class="flex justify-between items-center mb-6">
        <h2 class="panel-title">Inbox</h2>
        <div class="flex gap-1">
          {views.map((v) => (
            <button
              type="button"
              key={v}
              onClick={() => setActiveView(v)}
              class={`text-[0.72rem] font-medium py-1.5 px-3.5 rounded-md border font-sans cursor-pointer transition-all
                ${activeView === v
                  ? 'bg-surface-2 text-text border-border-light'
                  : 'bg-transparent text-text-dim border-border hover:bg-surface-2 hover:text-text hover:border-border-light'
                }`}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      {activeView === 'By Action' ? (
        groups.map((group) => (
          <InboxGroup key={group.id} group={group} />
        ))
      ) : (
        <div class="text-text-muted text-sm py-12 text-center">
          {activeView} view coming soon
        </div>
      )}
    </div>
  )
}
