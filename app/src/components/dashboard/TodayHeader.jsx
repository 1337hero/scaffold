const TodayHeader = () => {
  const now = new Date()
  const dateStr = now.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' })
  const start = new Date(now.getFullYear(), 0, 1)
  const diff = now - start
  const dayOfYear = Math.floor(diff / (1000 * 60 * 60 * 24))
  const totalDays = (now.getFullYear() % 4 === 0) ? 366 : 365
  const progress = Math.round((dayOfYear / totalDays) * 100)

  return (
    <header>
      <h2 class="text-4xl font-serif italic font-semibold">{dateStr}</h2>
      <div class="mt-4 max-w-xs">
        <div class="space-y-1.5">
          <div class="flex justify-between text-[10px] mono uppercase opacity-50">
            <span>Day {dayOfYear} / {totalDays}</span>
            <span>{progress}%</span>
          </div>
          <div class="h-2 w-full bg-black/5 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full"
              style={{ backgroundColor: '#1A1A1A', width: `${progress}%` }}
            />
          </div>
        </div>
      </div>
    </header>
  )
}

export default TodayHeader
