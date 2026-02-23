const DoneToday = ({ tasks = [] }) => {
  if (!tasks.length) return null

  return (
    <section class="space-y-4">
      <div class="flex items-center gap-4 opacity-40">
        <h3 class="text-xs font-bold mono uppercase tracking-widest">Done Today</h3>
        <div class="h-px flex-1 bg-app-border" />
      </div>
      <div class="space-y-2">
        {tasks.map(task => (
          <div key={task.ID} class="flex items-center gap-3 opacity-40 grayscale">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-emerald-500 shrink-0" aria-hidden="true">
              <circle cx="12" cy="12" r="10" /><path d="m9 12 2 2 4-4" />
            </svg>
            <span class="text-sm font-medium line-through">{task.Title}</span>
          </div>
        ))}
      </div>
    </section>
  )
}

export default DoneToday
