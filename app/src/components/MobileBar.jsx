import { cn } from "@/lib/utils.js"
import { desk, inbox, notebooks, search } from "@/constants/nav.js"

const tabs = [desk, inbox]

const rightTabs = [
  { ...notebooks, label: "Notes" },
  { ...search, disabled: true },
]

export function MobileBar({ activePanel, onNavigate, onCapture }) {
  return (
    <div class="md:hidden fixed bottom-0 left-0 right-0 bg-sidebar border-t border-border flex justify-around items-end pb-[max(8px,env(safe-area-inset-bottom))] pt-2 z-50">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => onNavigate(tab.id)}
          class={cn(
            "flex flex-col items-center gap-0.5 px-4 py-1.5 bg-transparent border-none font-sans text-[0.65rem] cursor-pointer transition-colors",
            activePanel === tab.id ? "text-amber" : "text-text-muted"
          )}
          aria-label={tab.label}
        >
          <tab.icon size={20} />
          {tab.label}
        </button>
      ))}

      <button
        onClick={onCapture}
        class="w-12 h-12 rounded-full bg-amber border-none text-bg text-[1.4rem] font-bold cursor-pointer -mt-4 shadow-[0_4px_12px_rgba(245,158,11,0.3)]"
        aria-label="Capture new item"
      >
        +
      </button>

      {rightTabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => !tab.disabled && onNavigate(tab.id)}
          class={cn(
            "flex flex-col items-center gap-0.5 px-4 py-1.5 bg-transparent border-none font-sans text-[0.65rem] cursor-pointer transition-colors",
            tab.disabled ? "text-text-muted opacity-40" : activePanel === tab.id ? "text-amber" : "text-text-muted"
          )}
          aria-label={tab.label}
          disabled={tab.disabled}
        >
          <tab.icon size={20} />
          {tab.label}
        </button>
      ))}
    </div>
  )
}
