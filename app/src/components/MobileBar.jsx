import { cn } from "@/lib/utils.js"
import { desk, inbox, map, search } from "@/constants/nav.js"

const tabs = [desk, inbox]

const rightTabs = [
  map,
  { ...search, disabled: true },
]

export function MobileBar({ activePanel, onNavigate, onCapture }) {
  return (
    <div class="md:hidden fixed bottom-0 left-0 right-0 bg-sidebar/95 border-t border-border flex justify-around items-end pb-[max(10px,env(safe-area-inset-bottom))] pt-2.5 z-50 backdrop-blur-sm">
      {tabs.map((tab) => (
        <button
          type="button"
          key={tab.id}
          onClick={() => onNavigate(tab.id)}
          class={cn(
            "flex flex-col items-center gap-1 px-4 py-1.5 bg-transparent border-none font-sans text-[0.72rem] cursor-pointer transition-colors",
            activePanel === tab.id ? "text-amber" : "text-text-muted"
          )}
          aria-label={tab.label}
        >
          <tab.icon size={22} />
          {tab.label}
        </button>
      ))}

      <button
        type="button"
        onClick={onCapture}
        class="w-14 h-14 rounded-full bg-amber border-none text-bg text-[1.55rem] font-bold cursor-pointer -mt-5 shadow-[0_4px_12px_rgba(245,158,11,0.3)]"
        aria-label="Capture new item"
      >
        +
      </button>

      {rightTabs.map((tab) => (
        <button
          type="button"
          key={tab.id}
          onClick={() => !tab.disabled && onNavigate(tab.id)}
          class={cn(
            "flex flex-col items-center gap-1 px-4 py-1.5 bg-transparent border-none font-sans text-[0.72rem] cursor-pointer transition-colors",
            tab.disabled ? "text-text-muted opacity-40" : activePanel === tab.id ? "text-amber" : "text-text-muted"
          )}
          aria-label={tab.label}
          disabled={tab.disabled}
        >
          <tab.icon size={22} />
          {tab.label}
        </button>
      ))}
    </div>
  )
}
