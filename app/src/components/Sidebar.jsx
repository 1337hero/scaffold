import { cn } from "@/lib/utils.js"
import { desk, inbox, map, search, graph } from "@/constants/nav.js"

const navItems = [
  desk,
  { ...inbox, hasBadge: true },
  map,
]

const toolItems = [search, graph]

export function Sidebar({ activePanel, onNavigate, onCapture, inboxCount }) {
  return (
    <aside class="hidden md:flex w-[264px] bg-sidebar border-r border-border flex-col py-7 shrink-0 h-screen sticky top-0">
      <div class="px-6 mb-9 flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-amber-dim border border-amber-border flex items-center justify-center text-[1rem] text-amber">
          {'\u26a1'}
        </div>
        <span class="text-[2rem] font-bold tracking-[-0.03em] leading-none">Scaffold</span>
      </div>

      <nav class="flex-1">
        {navItems.map((item) => (
          <button
            type="button"
            key={item.id}
            onClick={() => onNavigate(item.id)}
            class={cn(
              "flex items-center gap-3.5 w-full px-6 py-3 border-l-[3px] text-[1rem] font-medium transition-all cursor-pointer text-left",
              activePanel === item.id
                ? "bg-[rgba(245,158,11,0.06)] border-l-amber text-text"
                : "border-l-transparent text-text-dim hover:bg-[rgba(255,255,255,0.03)] hover:text-text"
            )}
            aria-label={item.label}
            aria-current={activePanel === item.id ? "page" : undefined}
          >
            <item.icon size={20} class={cn("shrink-0", activePanel === item.id ? "opacity-100" : "opacity-70")} />
            {item.label}
            {item.hasBadge && inboxCount > 0 && (
              <span class="ml-auto font-mono text-[0.72rem] font-semibold bg-amber-dim text-amber px-2 py-0.5 rounded-sm">
                {inboxCount}
              </span>
            )}
          </button>
        ))}

        <div class="text-[0.72rem] font-semibold uppercase tracking-[0.12em] text-text-muted px-6 pt-5 pb-2.5">
          Tools
        </div>

        {toolItems.map((item) => (
          <button
            type="button"
            key={item.id}
            class="flex items-center gap-3.5 w-full px-6 py-3 border-l-[3px] border-l-transparent text-text-dim text-[1rem] font-medium opacity-40 cursor-default text-left"
            disabled
            aria-label={`${item.label} (coming soon)`}
          >
            <item.icon size={20} class="shrink-0 opacity-70" />
            {item.label}
          </button>
        ))}
      </nav>

      <div class="px-5 pt-5 border-t border-border">
        <button
          type="button"
          onClick={onCapture}
          class="btn-amber w-full py-3 px-4 rounded-lg text-[0.95rem] flex items-center justify-center gap-2"
          aria-label="Capture new item"
        >
          + Capture
        </button>
      </div>
    </aside>
  )
}
