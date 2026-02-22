import { typeStyles } from '../../constants/colors.js'

export function InboxCard({ item, onConfirm, onOverride, onArchive, actionPending }) {
  const typeClass = typeStyles[item.type] || 'bg-surface-2 text-text-dim'
  const confirmDisabled = actionPending || item.confirmed
  const actionDisabled = actionPending

  return (
    <div class="surface-card py-5 px-6 mb-2 cursor-pointer hover:bg-surface-2 flex items-start gap-4 transition-all">
      <span class={`text-[0.68rem] font-semibold uppercase tracking-[0.08em] py-0.5 px-2.5 rounded-sm shrink-0 mt-0.5 ${typeClass}`}>
        {item.type}
      </span>

      <div class="flex-1 min-w-0">
        <div class="text-[1.06rem] leading-tight font-semibold mb-1.5 whitespace-nowrap overflow-hidden text-ellipsis">
          {item.title}
        </div>
        <div class="text-[0.92rem] text-text-dim leading-[1.45] line-clamp-2">
          {item.summary || 'Captured from inbox. Ready to triage.'}
        </div>

        <div class="flex gap-1.5 mt-3.5">
          <button
            type="button"
            disabled={confirmDisabled}
            onClick={() => onConfirm?.(item)}
            class={`text-[0.78rem] font-medium py-1.5 px-3 rounded-[6px] border font-sans cursor-pointer transition-all
              ${item.confirmed
                ? 'bg-green-dim border-green text-green cursor-default'
                : 'bg-green-dim border-green/40 text-green hover:bg-green/20'
              } disabled:opacity-70 disabled:cursor-not-allowed`}
          >
            {item.confirmed ? 'Confirmed' : 'Confirm'}
          </button>

          <button
            type="button"
            disabled={actionDisabled}
            onClick={() => onOverride?.(item)}
            class="text-[0.78rem] font-medium py-1.5 px-3 rounded-[6px] border border-purple/40 text-purple bg-purple-dim hover:bg-purple/20 font-sans cursor-pointer transition-all disabled:opacity-70 disabled:cursor-not-allowed"
          >
            Override
          </button>

          <button
            type="button"
            disabled={actionDisabled}
            onClick={() => onArchive?.(item)}
            class="btn-amber text-[0.78rem] py-1.5 px-3 rounded-[6px]"
          >
            Archive
          </button>
          </div>
      </div>

      <span class="font-mono text-[0.76rem] text-text-muted shrink-0 mt-1.5">
        {item.time}
      </span>
    </div>
  )
}
