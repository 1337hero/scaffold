import { coderNav, dashboard, inbox, notebooks, search } from "@/constants/nav.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine } from "@remixicon/react";

const navItems = [dashboard, { ...inbox, hasBadge: true }, notebooks, search, coderNav];

const Sidebar = ({ activeRoute, onNavigate, onCapture, inboxCount, coderActive }) => {
  return (
    <aside className="fixed left-0 top-0 h-full w-64 border-r border-app-border bg-[#1C1712] z-40 hidden lg:flex flex-col p-6 text-[#9C8E7A]">
      <div className="mb-10">
        <h1 className="font-serif italic text-2xl font-semibold tracking-tight text-[#F5F0E8]">
          Scaffold
        </h1>
        <p className="text-[10px] mono uppercase opacity-40 mt-1">
          Life Operating System
        </p>
      </div>

      <nav class="flex-1 space-y-2">
        {navItems.map((item) => {
          const active = activeRoute === item.id;
          return (
            <button
              type="button"
              key={item.id}
              onClick={() => onNavigate(item.path)}
              class={cn(
                "w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 group relative",
                active
                  ? "bg-[#2E2318] text-[#F5F0E8] shadow-lg shadow-black/5"
                  : "hover:bg-white/5 hover:text-[#F5F0E8]",
              )}
              aria-label={item.label}
              aria-current={active ? "page" : undefined}
            >
              <item.icon size={20} class="shrink-0" />
              <span class="font-medium">{item.label}</span>
              {item.hasBadge && inboxCount > 0 && (
                <span class="ml-auto px-2 py-0.5 rounded-full text-[10px] font-bold mono bg-[#F5F0E8] text-[#1C1712]">
                  {inboxCount}
                </span>
              )}
              {item.id === "coder" && coderActive && (
                <span class="ml-auto w-2 h-2 rounded-full bg-[#C47D3A] animate-pulse" />
              )}
              {active && (
                <div class="absolute left-0 w-1 h-6 bg-[#C47D3A] rounded-full ml-1 animate-indicator-appear" />
              )}
            </button>
          );
        })}
      </nav>

      <button
        type="button"
        onClick={onCapture}
        class="mt-auto flex items-center justify-center gap-2 w-full py-4 bg-[#C47D3A] hover:bg-[#B06A2E] text-white rounded-2xl font-bold shadow-lg shadow-[#C47D3A]/20 transition-all active:scale-95"
        aria-label="Capture new item"
      >
        <RiAddLine size={20} />
        <span>Capture</span>
        <span class="text-[10px] opacity-60 mono ml-1">⌘K</span>
      </button>
    </aside>
  );
};

export default Sidebar;
