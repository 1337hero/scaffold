import { RiAlarmLine, RiCakeLine, RiChatFollowUpLine, RiBookmarkLine } from "@remixicon/react";

const notificationTypes = {
  reminder: { icon: RiAlarmLine, href: () => "#/tasks", color: "text-status-warning" },
  birthday: { icon: RiCakeLine, href: () => "#/people", color: "text-status-error" },
  follow_up: { icon: RiChatFollowUpLine, href: () => "#/people", color: "text-status-info" },
  review: { icon: RiBookmarkLine, href: () => "#/library", color: "text-status-success" },
};

function birthdayWhen(daysUntil) {
  if (daysUntil === 0) return "today";
  if (daysUntil === 1) return "tomorrow";
  return `in ${daysUntil} days`;
}

const NotificationsSection = ({ notifications }) => {
  if (notifications.length === 0) return null;

  return (
    <section aria-label="Notifications" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3">Notifications</h2>
      <ul class="space-y-1">
        {notifications.map((n) => {
          const type = notificationTypes[n.type] ?? notificationTypes.reminder;
          return (
            <li key={`${n.type}-${n.ref_id}`}>
              <a
                href={type.href(n)}
                class="flex items-center gap-3 py-2 px-1 rounded hover:bg-app-bg transition-colors"
              >
                <type.icon size={18} class={`shrink-0 ${type.color}`} />
                <span class="text-sm font-medium">{n.title}</span>
                {n.type === "birthday" && n.days_until != null && (
                  <span class="text-xs text-app-muted">
                    {birthdayWhen(n.days_until)} · {n.date}
                  </span>
                )}
                {n.detail && <span class="text-xs text-app-muted truncate">{n.detail}</span>}
              </a>
            </li>
          );
        })}
      </ul>
    </section>
  );
};

export default NotificationsSection;
