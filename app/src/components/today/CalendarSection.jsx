import { RiCalendarLine } from "@remixicon/react";

const CalendarSection = ({ events }) => {
  return (
    <section aria-label="Calendar" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3 flex items-center gap-2">
        <RiCalendarLine size={16} /> Today's Calendar
      </h2>
      {events.length === 0 ? (
        <p class="text-sm text-app-muted">Nothing scheduled today.</p>
      ) : (
        <ul class="space-y-2">
          {events.map((e) => (
            <li key={e.id} class="flex items-baseline gap-3">
              <span class="text-xs mono text-app-muted w-16 shrink-0">{e.time}</span>
              <span class="text-sm">{e.summary}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
};

export default CalendarSection;
