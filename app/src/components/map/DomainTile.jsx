import DriftLabel from './DriftLabel.jsx'

const DomainTile = ({ domain, onOpenDomain }) => {
  const isDump = domain.id === 0 || domain.id === "dump"

  return (
    <button
      type="button"
      onClick={() => onOpenDomain(isDump ? "dump" : domain.id)}
      class="surface-card p-4 text-left cursor-pointer w-full flex flex-col gap-1.5"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="text-[0.95rem] font-semibold truncate">{domain.name}</span>
        {domain.open_task_count > 0 && (
          <span class="text-[0.72rem] font-mono text-text-muted shrink-0">
            {domain.open_task_count} open
          </span>
        )}
      </div>

      <DriftLabel state={domain.drift_state} label={domain.drift_label} />

      {!isDump && domain.days_since_touch != null && (
        <span class="text-[0.75rem] text-text-dim">
          {domain.days_since_touch === 0 ? "Touched today" : `${domain.days_since_touch}d ago`}
        </span>
      )}

      {domain.status_line && (
        <span class="text-[0.78rem] text-text-dim truncate">{domain.status_line}</span>
      )}
    </button>
  )
}

export default DomainTile
