import { typeStyles } from '../../data/mock.js'

export function InboxCard({ item }) {
  const typeClass = typeStyles[item.type] || 'bg-surface-2 text-text-dim'

  return (
    <div class="bg-surface border border-border rounded-[10px] py-4 px-5 mb-1.5 cursor-pointer transition-all hover:border-border-light hover:bg-surface-2 flex items-start gap-3.5">
      <span class={`text-[0.6rem] font-semibold uppercase tracking-[0.06em] py-[3px] px-2 rounded-sm shrink-0 mt-0.5 ${typeClass}`}>
        {item.type}
      </span>

      <div class="flex-1 min-w-0">
        <div class="text-[0.9rem] font-semibold mb-[3px] whitespace-nowrap overflow-hidden text-ellipsis">
          {item.title}
        </div>
        <div class="text-[0.78rem] text-text-dim leading-relaxed line-clamp-2">
          {item.summary}
        </div>

        {item.actions?.length > 0 && (
          <div class="flex gap-1 mt-2.5">
            {item.actions.map((action, i) => (
              <button
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

      <span class="font-mono text-[0.65rem] text-text-muted shrink-0 mt-[3px]">
        {item.time}
      </span>
    </div>
  )
}
