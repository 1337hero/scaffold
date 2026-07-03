import { navItems } from "@/constants/nav.js";
import { cn } from "@/lib/utils.js";
import { RiSearchLine } from "@remixicon/react";

const mobileItems = [...navItems, { id: "search", path: "#/search", icon: RiSearchLine, label: "Search" }];

const MobileBar = ({ activeRoute }) => {
  return (
    <nav class="fixed bottom-0 left-0 w-full h-20 bg-card-bg border-t border-app-border z-40 lg:hidden flex items-center justify-around px-1 pb-2">
      {mobileItems.map((item) => {
        const active = activeRoute === item.id;
        return (
          <a
            key={item.id}
            href={item.path}
            class={cn("relative p-2.5 transition-all", active ? "text-app-ink" : "text-app-muted")}
            aria-label={item.label}
            aria-current={active ? "page" : undefined}
          >
            <item.icon size={24} />
          </a>
        );
      })}
    </nav>
  );
};

export default MobileBar;
