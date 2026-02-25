import { agentsNav, dashboard, inbox, areas, search } from "@/constants/nav.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine, RiLogoutBoxRLine } from "@remixicon/react";

const navItems = [dashboard, { ...inbox, hasBadge: true }, areas, search, agentsNav];

const Sidebar = ({ activeRoute, onNavigate, onCapture, onLogout, inboxCount, coderActive }) => {
  return (
    <aside className="fixed left-0 top-0 h-full w-64 border-r border-app-border bg-sidebar-bg z-40 hidden lg:flex flex-col p-6 text-sidebar-text">
      <div className="mb-10">
        <h1 className="font-serif italic text-2xl font-semibold tracking-tight text-app-bg">
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
                  ? "bg-sidebar-active text-app-bg shadow-lg shadow-black/5"
                  : "hover:bg-white/5 hover:text-app-bg",
              )}
              aria-label={item.label}
              aria-current={active ? "page" : undefined}
            >
              <item.icon size={20} class="shrink-0" />
              <span class="font-medium">{item.label}</span>
              {item.hasBadge && inboxCount > 0 && (
                <span class="ml-auto px-2 py-0.5 rounded-full text-[10px] font-bold mono bg-app-bg text-sidebar-bg">
                  {inboxCount}
                </span>
              )}
              {item.id === "agents" && coderActive && (
                <span class="ml-auto w-2 h-2 rounded-full bg-accent animate-pulse" />
              )}
              {active && (
                <div class="absolute left-0 w-1 h-6 bg-accent rounded-full ml-1 animate-indicator-appear" />
              )}
            </button>
          );
        })}
      </nav>

      <div class="mt-auto flex flex-col gap-3">
        <button
          type="button"
          onClick={onLogout}
          title="Log out"
          class="flex items-center gap-3 px-4 py-2 rounded-xl text-sidebar-text/50 hover:text-sidebar-text hover:bg-white/5 transition-all duration-200 w-full"
          aria-label="Log out"
        >
          <RiLogoutBoxRLine size={18} class="shrink-0" />
          <span class="text-sm">Log out</span>
        </button>

        <button
          type="button"
          onClick={onCapture}
          class="flex items-center justify-center gap-2 w-full py-4 bg-accent hover:bg-accent-hover text-white rounded-2xl font-bold shadow-lg shadow-accent/20 transition-all active:scale-95"
          aria-label="Capture new item"
        >
          <RiAddLine size={20} />
          <span>Capture</span>
          <span class="text-[10px] opacity-60 mono ml-1">⌘K</span>
        </button>
      </div>
    </aside>
  );
};

export default Sidebar;
