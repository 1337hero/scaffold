import { cn } from "@/lib/utils.js";
import { RiLayoutGridLine, RiListUnordered, RiSearchLine } from "@remixicon/react";
import { relationshipOptions } from "./personUtils.js";

const selectClass =
  "px-3 py-1.5 rounded-full bg-card-bg border border-app-border text-sm text-app-ink focus:outline-none focus:border-accent";

const PeopleFilters = ({ filters, onChange, domains, view, onViewChange }) => {
  const set = (key) => (e) => onChange({ ...filters, [key]: e.currentTarget.value });

  return (
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-[16rem] flex-1 sm:flex-none">
        <RiSearchLine size={15} class="absolute left-3 top-1/2 -translate-y-1/2 text-app-muted" />
        <input
          type="search"
          name="people-search"
          value={filters.search}
          onInput={set("search")}
          placeholder="Search people…"
          aria-label="Search people"
          autocomplete="off"
          class="w-full pl-9 pr-3 py-1.5 rounded-full bg-card-bg border border-app-border text-sm focus:outline-none focus:border-accent"
        />
      </div>

      <select value={filters.relationship} onChange={set("relationship")} class={selectClass} aria-label="Relationship">
        <option value="">All relationships</option>
        {relationshipOptions.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>

      <select value={filters.domainId} onChange={set("domainId")} class={selectClass} aria-label="Domain">
        <option value="">All domains</option>
        {domains.map((domain) => (
          <option key={domain.ID} value={String(domain.ID)}>
            {domain.Name}
          </option>
        ))}
      </select>

      <div class="ml-auto inline-flex rounded-full border border-app-border bg-card-bg p-0.5">
        <button
          type="button"
          onClick={() => onViewChange("grid")}
          aria-label="Grid view"
          aria-pressed={view === "grid"}
          class={cn(
            "flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold transition-colors",
            view === "grid" ? "bg-accent text-white" : "text-app-muted hover:text-app-ink",
          )}
        >
          <RiLayoutGridLine size={14} /> Grid
        </button>
        <button
          type="button"
          onClick={() => onViewChange("list")}
          aria-label="List view"
          aria-pressed={view === "list"}
          class={cn(
            "flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold transition-colors",
            view === "list" ? "bg-accent text-white" : "text-app-muted hover:text-app-ink",
          )}
        >
          <RiListUnordered size={14} /> List
        </button>
      </div>
    </div>
  );
};

export default PeopleFilters;
