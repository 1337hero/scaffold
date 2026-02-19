import { cn } from "@/lib/utils.js"
import { desk, inbox, notebooks, search, graph } from "@/constants/nav.js"

const navItems = [
  desk,
  { ...inbox, hasBadge: true },
  notebooks,
]

const toolItems = [search, graph]

export function Sidebar({ activePanel, onNavigate, onCapture, inboxCount }) {
  return (
    <aside class="hidden md:flex w-[220px] bg-sidebar border-r border-border flex-col py-5 shrink-0 h-screen sticky top-0">
      <div class="px-5 mb-7 flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-md bg-amber-dim flex items-center justify-center text-[0.9rem]">
          {'\u26a1'}
        </div>
        <span class="text-[1.1rem] font-bold tracking-tight">Scaffold</span>
      </div>

      <nav class="flex-1">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            class={cn(
              "flex items-center gap-3 w-full px-5 py-2.5 border-l-[3px] text-[0.88rem] font-medium transition-all cursor-pointer",
              activePanel === item.id
                ? "bg-[rgba(245,158,11,0.06)] border-l-amber text-text"
                : "border-l-transparent text-text-dim hover:bg-[rgba(255,255,255,0.03)] hover:text-text"
            )}
            aria-label={item.label}
          >
            <item.icon size={18} class={cn("shrink-0", activePanel === item.id ? "opacity-100" : "opacity-70")} />
            {item.label}
            {item.hasBadge && inboxCount > 0 && (
              <span class="ml-auto font-mono text-[0.62rem] font-semibold bg-amber-dim text-amber px-1.5 py-0.5 rounded-sm">
                {inboxCount}
              </span>
            )}
          </button>
        ))}

        <div class="text-[0.6rem] font-semibold uppercase tracking-[0.12em] text-text-muted px-5 pt-4 pb-2">
          Tools
        </div>

        {toolItems.map((item) => (
          <button
            key={item.id}
            class="flex items-center gap-3 w-full px-5 py-2.5 border-l-[3px] border-l-transparent text-text-dim text-[0.88rem] font-medium opacity-40 cursor-default"
            disabled
            aria-label={`${item.label} (coming soon)`}
          >
            <item.icon size={18} class="shrink-0 opacity-70" />
            {item.label}
          </button>
        ))}
      </nav>

      <div class="px-4 pt-4 border-t border-border">
        <button
          onClick={onCapture}
          class="w-full py-2.5 px-4 bg-amber-dim border border-amber-border text-amber rounded-md font-sans text-[0.82rem] font-semibold cursor-pointer transition-all hover:bg-[rgba(245,158,11,0.18)] flex items-center justify-center gap-2"
          aria-label="Capture new item"
        >
          + Capture
        </button>
      </div>
    </aside>
  )
}
