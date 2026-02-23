import { useQuery } from '@tanstack/react-query'
import { calendarQuery } from '@/api/queries.js'

const CalendarEvents = () => {
  const { data: events = [] } = useQuery(calendarQuery)
  if (!events.length) return null

  return (
    <div class="mb-6 flex flex-col gap-1.5">
      {events.slice(0, 3).map((event) => (
        <div key={event.id} class="flex items-center gap-2.5 text-[0.82rem] text-text-dim">
          <span class="text-text-muted">{event.time}</span>
          <span>{event.summary}</span>
        </div>
      ))}
    </div>
  )
}

export default CalendarEvents
