import { navItems } from "@/constants/nav.js";
import { SURFACES } from "@/constants/surfaces.js";
import { useSurface } from "@/hooks/useSurface.jsx";
import { cn } from "@/lib/utils.js";
import { RiLogoutBoxRLine, RiSearchLine } from "@remixicon/react";

const Sidebar = ({ activeRoute, onLogout }) => {
  const { surface } = useSurface();

  return (
    <aside class="fixed left-0 top-0 h-full w-64 border-r border-app-border bg-sidebar-bg z-40 hidden lg:flex flex-col p-6 text-sidebar-text">
      <div class="mb-10">
        <h1 class="font-serif italic text-2xl font-semibold tracking-tight text-app-bg">
          Scaffold
        </h1>
        <p class="text-[10px] mono uppercase opacity-40 mt-1">{SURFACES[surface].tagline}</p>
      </div>

      <nav class="flex-1 space-y-2">
        {navItems.map((item) => {
          const active = activeRoute === item.id;
          return (
            <a
              key={item.id}
              href={item.path}
              class={cn(
                "flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 group relative",
                active
                  ? "bg-sidebar-active text-app-bg shadow-lg shadow-black/5"
                  : "hover:bg-white/5 hover:text-app-bg",
              )}
              aria-current={active ? "page" : undefined}
            >
              <item.icon size={20} class="shrink-0" />
              <span class="font-medium">{item.label}</span>
              {active && (
                <div class="absolute left-0 w-1 h-6 bg-accent rounded-full ml-1 animate-indicator-appear" />
              )}
            </a>
          );
        })}
      </nav>

      <div class="mt-auto flex flex-col gap-1">
        <a
          href="#/search"
          class={cn(
            "flex items-center gap-3 px-4 py-2 rounded-xl transition-all duration-200",
            activeRoute === "search"
              ? "text-sidebar-text bg-white/5"
              : "text-sidebar-text/50 hover:text-sidebar-text hover:bg-white/5",
          )}
          aria-current={activeRoute === "search" ? "page" : undefined}
        >
          <RiSearchLine size={18} class="shrink-0" />
          <span class="text-sm">Search</span>
        </a>

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
      </div>
    </aside>
  );
};

export default Sidebar;
