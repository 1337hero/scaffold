import { InboxCard } from './InboxCard.jsx'
import { colorClass } from '../../data/mock.js'

export function InboxGroup({ group }) {
  return (
    <div class="mb-6">
      <div class="flex items-center gap-2.5 mb-2.5 cursor-default">
        <div class={`w-2 h-2 rounded-full ${colorClass(group.color, 'dot')}`} />
        <span class="text-[0.82rem] font-semibold text-text">{group.label}</span>
        <span class="font-mono text-[0.65rem] text-text-muted">{group.items.length} items</span>
      </div>

      {group.items.map((item) => (
        <InboxCard key={item.id} item={item} />
      ))}
    </div>
  )
}
