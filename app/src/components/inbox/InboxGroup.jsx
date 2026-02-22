import { InboxCard } from './InboxCard.jsx'
import { colorClass } from '../../constants/colors.js'

export function InboxGroup({ group, onConfirm, onOverride, onArchive, actionPending }) {
  return (
    <div class="mb-8">
      <div class="flex items-center gap-3 mb-3.5 cursor-default">
        <div class={`w-2.5 h-2.5 rounded-full ${colorClass(group.color, 'dot')}`} />
        <span class="text-[1.05rem] font-semibold text-text">{group.label}</span>
        <span class="font-mono text-[0.78rem] text-text-muted">{group.items.length} items</span>
      </div>

      {group.items.map((item) => (
        <InboxCard
          key={item.id}
          item={item}
          onConfirm={onConfirm}
          onOverride={onOverride}
          onArchive={onArchive}
          actionPending={actionPending}
        />
      ))}
    </div>
  )
}
