import { daysSince } from "@/lib/utils.js";
import { RiAlarmWarningLine } from "@remixicon/react";

const SlippingRow = ({ href, name, detail }) => (
  <li>
    <a href={href} class="flex items-baseline justify-between gap-3 py-1.5 hover:bg-app-bg rounded px-1 transition-colors">
      <span class="text-sm truncate">{name}</span>
      <span class="text-xs text-status-warning shrink-0">{detail}</span>
    </a>
  </li>
);

const groups = [
  {
    key: "tasks",
    label: "Tasks",
    href: () => "#/tasks",
    name: (t) => t.title,
    detail: (t) => `${t.daysOverdue}d overdue`,
  },
  {
    key: "projects",
    label: "Projects",
    href: () => "#/projects",
    name: (p) => p.name,
    detail: (p) => (p.lastActivityAt ? `${daysSince(p.lastActivityAt)}d quiet` : "no activity yet"),
  },
  {
    key: "areas",
    label: "Areas",
    href: () => "#/projects",
    name: (a) => a.name,
    detail: (a) => (a.lastActivityAt ? `${daysSince(a.lastActivityAt)}d quiet` : "no activity yet"),
  },
  {
    key: "people",
    label: "People",
    href: () => "#/people",
    name: (p) => p.name,
    detail: (p) => `${daysSince(p.lastInteractionAt)}d since contact`,
  },
];

const SlippingSection = ({ slipping }) => {
  const total = groups.reduce((n, g) => n + slipping[g.key].length, 0);

  return (
    <section aria-label="Slipping" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3 flex items-center gap-2">
        <RiAlarmWarningLine size={16} /> Slipping
      </h2>
      {total === 0 ? (
        <p class="text-sm text-app-muted">Nothing is slipping. Clean slate.</p>
      ) : (
        <div class="space-y-4">
          {groups.map((g) => {
            const items = slipping[g.key];
            if (items.length === 0) return null;
            return (
              <div key={g.key}>
                <h3 class="text-xs font-semibold text-app-muted mb-1">{g.label}</h3>
                <ul>
                  {items.map((item) => (
                    <SlippingRow
                      key={item.id}
                      href={g.href(item)}
                      name={g.name(item)}
                      detail={g.detail(item)}
                    />
                  ))}
                </ul>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
};

export default SlippingSection;
