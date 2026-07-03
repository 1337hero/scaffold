import { cn, daysSince } from "@/lib/utils.js";
import { RiSearchLine } from "@remixicon/react";
import { useState } from "preact/hooks";

const typeGroups = [
  { type: "project", label: "Projects" },
  { type: "area", label: "Areas" },
  { type: "retainer", label: "Retainers" },
];

const statusRank = { active: 0, on_hold: 1, completed: 2, archived: 3 };

const ProjectSidebar = ({ projects, activeId }) => {
  const [search, setSearch] = useState("");

  const visible = projects
    .filter((p) => !search || p.name.toLowerCase().includes(search.toLowerCase()))
    .sort((a, b) => (statusRank[a.status] ?? 9) - (statusRank[b.status] ?? 9) || a.name.localeCompare(b.name));

  return (
    <div class="space-y-4">
      <div class="relative">
        <RiSearchLine size={15} class="absolute left-3 top-1/2 -translate-y-1/2 text-app-muted" />
        <input
          type="search"
          value={search}
          onInput={(e) => setSearch(e.currentTarget.value)}
          placeholder="Filter by name"
          class="w-full pl-9 pr-3 py-2 rounded-full bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
        />
      </div>

      {typeGroups.map((g) => {
        const items = visible.filter((p) => p.type === g.type);
        if (items.length === 0) return null;
        return (
          <div key={g.type}>
            <h3 class="text-xs font-bold uppercase tracking-wide text-app-muted mb-1">{g.label}</h3>
            <ul class="space-y-0.5">
              {items.map((p) => {
                const slipping = p.status === "active" && p.type !== "retainer" &&
                  (!p.lastActivityAt || daysSince(p.lastActivityAt) >= (p.type === "area" ? 14 : 7));
                return (
                  <li key={p.id}>
                    <a
                      href={`#/projects/${p.id}`}
                      class={cn(
                        "flex items-center justify-between gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors",
                        activeId === p.id ? "bg-card-bg border border-app-border font-medium" : "hover:bg-card-bg/60",
                        p.status !== "active" && "text-app-muted",
                      )}
                      aria-current={activeId === p.id ? "page" : undefined}
                    >
                      <span class="truncate">{p.name}</span>
                      <span class="flex items-center gap-1.5 shrink-0">
                        {slipping && <span class="w-2 h-2 rounded-full bg-status-warning" title="Slipping" />}
                        {p.status !== "active" && <span class="text-[10px] uppercase">{p.status.replace("_", " ")}</span>}
                      </span>
                    </a>
                  </li>
                );
              })}
            </ul>
          </div>
        );
      })}
    </div>
  );
};

export default ProjectSidebar;
