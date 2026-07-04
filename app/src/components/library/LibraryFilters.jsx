import { RiSearchLine } from "@remixicon/react";

const inputClass =
  "px-3 py-1.5 rounded-full bg-card-bg border border-app-border text-sm text-app-ink focus:outline-none focus:border-accent";

const LibraryFilters = ({ kind, filters, onChange }) => {
  const set = (key) => (e) => onChange({ ...filters, [key]: e.currentTarget.value });

  return (
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-[16rem] flex-1 sm:flex-none">
        <RiSearchLine size={15} class="absolute left-3 top-1/2 -translate-y-1/2 text-app-muted" />
        <input
          type="search"
          name="library-search"
          value={filters.q}
          onInput={set("q")}
          placeholder={`Search ${kind === "quote" ? "quotes" : kind === "journal" ? "journal" : "notes"}…`}
          aria-label="Search library"
          autocomplete="off"
          class="w-full rounded-full border border-app-border bg-card-bg py-1.5 pl-9 pr-3 text-sm focus:outline-none focus:border-accent"
        />
      </div>

      {kind !== "journal" && (
        <input
          type="text"
          name="source"
          value={filters.source}
          onInput={set("source")}
          placeholder="Source…"
          aria-label="Source"
          autocomplete="off"
          class={inputClass}
        />
      )}

      {kind === "note" && (
        <>
          <input
            type="text"
            name="tags"
            value={filters.tags}
            onInput={set("tags")}
            placeholder="Tag…"
            aria-label="Tag"
            autocomplete="off"
            class={inputClass}
          />
          <select value={filters.flagForReview} onChange={set("flagForReview")} class={inputClass} aria-label="Review">
            <option value="">All review states</option>
            <option value="true">Flagged</option>
            <option value="false">Unflagged</option>
          </select>
        </>
      )}

      {kind === "journal" && (
        <>
          <input
            type="date"
            name="from"
            value={filters.from}
            onInput={set("from")}
            aria-label="From date"
            class={inputClass}
          />
          <input
            type="date"
            name="to"
            value={filters.to}
            onInput={set("to")}
            aria-label="To date"
            class={inputClass}
          />
        </>
      )}
    </div>
  );
};

export default LibraryFilters;
