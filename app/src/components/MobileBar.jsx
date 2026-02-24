import { useQuery } from "@tanstack/react-query"
import { inboxCountQuery } from "@/api/queries.js"
import { dashboard, inbox, areas, search } from "@/constants/nav.js"

const DashboardIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect width="7" height="9" x="3" y="3" rx="1" />
    <rect width="7" height="5" x="3" y="16" rx="1" />
    <rect width="7" height="5" x="14" y="3" rx="1" />
    <rect width="7" height="9" x="14" y="12" rx="1" />
  </svg>
)

const InboxIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polyline points="22 12 16 12 14 15 10 15 8 12 2 12" />
    <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
  </svg>
)

const PlusIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
    <path d="M5 12h14" />
    <path d="M12 5v14" />
  </svg>
)

const AreasIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
    <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
  </svg>
)

const SearchIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="11" cy="11" r="8" />
    <path d="m21 21-4.35-4.35" />
  </svg>
)

const MobileBar = ({ activeRoute, onNavigate, onCapture }) => {
  const { data: inboxCount = 0 } = useQuery(inboxCountQuery)

  const navItems = [
    { id: dashboard.id, path: dashboard.path, icon: DashboardIcon },
    { id: inbox.id, path: inbox.path, icon: InboxIcon, badge: inboxCount },
    { id: "capture", icon: PlusIcon, primary: true },
    { id: areas.id, path: areas.path, icon: AreasIcon },
    { id: search.id, path: search.path, icon: SearchIcon },
  ]

  return (
    <nav class="fixed bottom-0 left-0 w-full h-20 bg-[var(--color-card-bg)] border-t border-app-border z-40 lg:hidden flex items-center justify-around px-4 pb-2">
      {navItems.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={() =>
            item.primary ? onCapture() : onNavigate(item.path)
          }
          class={item.primary
            ? "relative p-3 transition-all cursor-pointer border-none bg-emerald-500 text-white rounded-2xl -translate-y-4 shadow-xl shadow-emerald-500/30"
            : `relative p-3 transition-all cursor-pointer border-none bg-transparent ${activeRoute === item.id ? "text-app-ink" : "text-app-muted"}`
          }
          aria-label={item.id}
        >
          <item.icon />
          {item.badge > 0 && (
            <span class="absolute top-1 right-1 w-5 h-5 bg-app-ink text-[var(--color-card-bg)] text-[10px] font-bold flex items-center justify-center rounded-full border-2 border-[var(--color-card-bg)]">
              {item.badge}
            </span>
          )}
        </button>
      ))}
    </nav>
  )
}

export default MobileBar
