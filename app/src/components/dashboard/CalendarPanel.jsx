function formatEventTime(event) {
  if (event?.time) return event.time
  if (event?.all_day) return "All day"
  if (!event?.start) return ""
  const d = new Date(event.start)
  return d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }).toLowerCase()
}

function eventSummary(event) {
  return event?.summary || event?.title || event?.Title || "Untitled"
}

const CalendarPanel = ({ events = [] }) => {
  const list = Array.isArray(events) ? events : []

  return (
    <section class="space-y-4">
      <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">Calendar</h3>
      <div class="bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow p-4 space-y-4">
        {list.length ? (
          list.map((event, i) => {
            const key = event?.id || event?.ID || i
            const href = event?.htmlLink || null
            const time = formatEventTime(event)

            const row = (
              <>
                <span class="text-[10px] mono opacity-50 w-14 text-right">{time}</span>
                <div class="w-1 h-1 rounded-full bg-app-ink opacity-20" />
                <span class="text-sm font-medium">{eventSummary(event)}</span>
              </>
            )

            if (href) {
              return (
                <a
                  key={key}
                  href={href}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-center gap-3 hover:opacity-70 transition-opacity"
                >
                  {row}
                </a>
              )
            }

            return (
              <div key={key} class="flex items-center gap-3">
                {row}
              </div>
            )
          })
        ) : (
          <div class="py-4 text-center opacity-30">
            <p class="font-serif italic text-sm">No events today</p>
          </div>
        )}
      </div>
    </section>
  )
}

export default CalendarPanel
