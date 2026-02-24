import { useState, useEffect } from "preact/hooks"
import { useQuery } from "@tanstack/react-query"
import { searchQuery, domainsQuery } from "@/api/queries.js"
import { nullable } from "@/utils/nullable.js"
import { typeColor } from "@/constants/colors.js"

const FILTERS = [
  { id: "", label: "All" },
  { id: "goal", label: "Goals" },
  { id: "task", label: "Tasks" },
  { id: "note", label: "Notes" },
]

const SearchIcon = ({ size = 24 }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
  >
    <circle cx="11" cy="11" r="8" />
    <path d="m21 21-4.35-4.35" />
  </svg>
)

function ResultItem({ result, domains }) {
  const domainId = nullable(result.DomainID)
  const domain = domainId ? domains.find((d) => d.ID === domainId) : null
  const color = typeColor(result.Type)
  const href = domainId ? `#/notebooks/${domainId}` : null

  const card = (
    <div class="p-4 bg-[var(--color-card-bg)] rounded-2xl border border-app-border card-shadow hover:border-app-ink/10 transition-all flex items-center gap-3 cursor-pointer">
      <span
        class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 font-bold shrink-0"
        style={{ color }}
      >
        {result.Type}
      </span>
      <div class="flex-1 min-w-0">
        <p class="font-semibold text-sm leading-tight truncate">{result.Title}</p>
        {result.Snippet && (
          <p class="text-xs text-app-muted mt-0.5 truncate opacity-60">{result.Snippet}</p>
        )}
      </div>
      {domain && (
        <span
          class="text-[9px] mono uppercase px-1.5 py-0.5 rounded bg-black/5 opacity-60 shrink-0"
          style={{ color: typeColor('goal') }}
        >
          {domain.Name}
        </span>
      )}
    </div>
  )

  if (href) return <a href={href} class="no-underline text-inherit block">{card}</a>
  return card
}

const Search = () => {
  const [inputValue, setInputValue] = useState("")
  const [debouncedQuery, setDebouncedQuery] = useState("")
  const [typeFilter, setTypeFilter] = useState("")
  const { data: domains = [] } = useQuery(domainsQuery)

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(inputValue), 300)
    return () => clearTimeout(timer)
  }, [inputValue])

  const filters = typeFilter ? { type: typeFilter } : {}
  const { data: results = [], isLoading, isFetching } = useQuery(
    searchQuery(debouncedQuery, filters),
  )

  const hasQuery = debouncedQuery.trim().length > 0

  return (
    <div class="max-w-3xl mx-auto space-y-8">
      <div class="relative">
        <span class="absolute left-6 top-1/2 -translate-y-1/2 text-app-muted pointer-events-none">
          <SearchIcon size={24} />
        </span>
        <input
          type="text"
          value={inputValue}
          onInput={(e) => setInputValue(e.currentTarget.value)}
          placeholder="Search goals, tasks, notes..."
          class="w-full bg-[var(--color-card-bg)] border border-app-border rounded-3xl py-6 pl-16 pr-6 text-xl focus:outline-none focus:ring-2 focus:ring-app-ink/5 card-shadow"
        />
        {isFetching && (
          <div class="absolute right-6 top-1/2 -translate-y-1/2 w-4 h-4 border-2 border-app-ink/20 border-t-app-ink/60 rounded-full animate-spin" />
        )}
      </div>

      <div class="flex gap-2 overflow-x-auto pb-2">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            type="button"
            onClick={() => setTypeFilter(f.id)}
            class={`px-4 py-2 rounded-xl border text-xs font-bold transition-all shrink-0 cursor-pointer ${
              typeFilter === f.id
                ? "bg-app-ink text-white border-app-ink"
                : "bg-[var(--color-card-bg)] border-app-border text-app-muted hover:bg-app-ink hover:text-white hover:border-app-ink"
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {!hasQuery && (
        <div class="py-20 text-center opacity-20">
          <span class="flex justify-center mb-4">
            <SearchIcon size={48} />
          </span>
          <p class="font-serif italic text-lg">Start typing to search...</p>
        </div>
      )}

      {hasQuery && !isLoading && results.length === 0 && (
        <div class="py-20 text-center opacity-20">
          <span class="flex justify-center mb-4">
            <SearchIcon size={48} />
          </span>
          <p class="font-serif italic text-lg">No results for &ldquo;{debouncedQuery}&rdquo;</p>
        </div>
      )}

      {hasQuery && results.length > 0 && (
        <div class="space-y-3">
          {results.map((result) => (
            <ResultItem key={result.ID} result={result} domains={domains} />
          ))}
        </div>
      )}
    </div>
  )
}

export default Search
