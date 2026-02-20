import { typeStyles } from '../../data/mock.js'

export function InboxCard({ item }) {
  const typeClass = typeStyles[item.type] || 'bg-surface-2 text-text-dim'
  const hasActions = item.actions?.length > 0

  return (
    <div class="surface-card py-4 px-5 mb-1.5 cursor-pointer hover:bg-surface-2 flex items-start gap-3.5 transition-all">
      <span class={`text-[0.6rem] font-semibold uppercase tracking-[0.06em] py-0.5 px-2 rounded-sm shrink-0 mt-0.5 ${typeClass}`}>
        {item.type}
      </span>

      <div class="flex-1 min-w-0">
        <div class="text-[0.9rem] leading-tight font-semibold mb-1 whitespace-nowrap overflow-hidden text-ellipsis">
          {item.title}
        </div>
        <div class="text-[0.78rem] text-text-dim leading-normal line-clamp-2">
          {item.summary || 'Captured from inbox. Ready to triage.'}
        </div>

        {hasActions && (
          <div class="flex gap-1 mt-2.5">
            {item.actions.map((action, i) => (
              <button
                type="button"
                key={action}
                class={`text-[0.68rem] font-medium py-1 px-2.5 rounded-[5px] border font-sans cursor-pointer transition-all
                  ${i === 0
                    ? 'bg-amber-dim border-amber-border text-amber hover:bg-[rgba(245,158,11,0.18)]'
                    : 'bg-transparent border-border text-text-dim hover:bg-surface-3 hover:text-text'
                  }`}
              >
                {action}
              </button>
            ))}
          </div>
        )}
      </div>

      <span class="font-mono text-[0.65rem] text-text-muted shrink-0 mt-1">
        {item.time}
      </span>
    </div>
  )
}
