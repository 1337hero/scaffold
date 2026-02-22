import { useState, useEffect, useRef } from 'preact/hooks'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { domainDetailQuery, patchDomain } from '@/api/queries.js'
import { DriftLabel } from './DriftLabel.jsx'

const norm = (raw, ...keys) => keys.reduce((v, k) => v ?? raw?.[k], undefined)

const normDeskItem = (raw) => ({
  id: norm(raw, 'ID', 'id'),
  title: norm(raw, 'Title', 'title') ?? '',
  status: norm(raw, 'Status', 'status') ?? null,
})

const normCapture = (raw) => ({
  id: norm(raw, 'ID', 'id'),
  title: norm(raw, 'Title', 'title', 'Raw', 'raw') ?? 'Untitled capture',
})

const normMemory = (raw) => ({
  id: norm(raw, 'ID', 'id'),
  title: norm(raw, 'Title', 'title', 'Body', 'body') ?? 'Untitled memory',
})

function ImportanceDots({ value }) {
  return (
    <span class="inline-flex gap-0.5" title={`Importance: ${value}/5`}>
      {Array.from({ length: 5 }, (_, i) => (
        <span
          key={i}
          class={`w-1.5 h-1.5 rounded-full ${i < value ? 'bg-amber' : 'bg-surface-3'}`}
        />
      ))}
    </span>
  )
}

function formatTouched(dateStr) {
  if (!dateStr) return ""
  const d = new Date(dateStr)
  if (isNaN(d)) return ""
  const now = new Date()
  const diffMs = now - d
  const days = Math.floor(diffMs / 86400000)
  if (days === 0) return "Today"
  if (days === 1) return "Yesterday"
  return `${days} days ago`
}

function Section({ label, children, empty }) {
  return (
    <div class="mb-6">
      <div class="flex items-center gap-3 mb-2.5">
        <span class="text-[0.72rem] font-semibold uppercase tracking-[0.12em] text-text-muted">
          {label}
        </span>
        <span class="flex-1 h-px bg-border" />
      </div>
      {children || (
        <div class="text-[0.85rem] text-text-dim py-2">{empty ?? "None"}</div>
      )}
    </div>
  )
}

function BriefingEditor({ domainId, initial }) {
  const queryClient = useQueryClient()
  const [text, setText] = useState(initial ?? "")
  const timerRef = useRef(null)
  const savedRef = useRef(initial ?? "")

  const mutation = useMutation({
    mutationFn: (briefing) => patchDomain(domainId, { briefing }),
    onSuccess: (_, briefing) => {
      savedRef.current = briefing
      queryClient.invalidateQueries({ queryKey: ["domain", domainId] })
      queryClient.invalidateQueries({ queryKey: ["domains"] })
    },
  })

  useEffect(() => {
    setText(initial ?? "")
    savedRef.current = initial ?? ""
  }, [initial])

  function scheduleAutoSave(value) {
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      timerRef.current = null
      mutation.mutate(value)
    }, 500)
  }

  function handleInput(e) {
    const val = e.target.value
    setText(val)
    scheduleAutoSave(val)
  }

  function handleBlur() {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (text !== savedRef.current) {
      mutation.mutate(text)
    }
  }

  return (
    <div>
      <textarea
        class="w-full min-h-[120px] bg-surface-2 border border-border rounded-lg p-3.5 text-[0.88rem] text-text font-sans leading-relaxed resize-y focus:border-amber focus:outline-none transition-colors"
        value={text}
        onInput={handleInput}
        onBlur={handleBlur}
        placeholder="Write a briefing for this domain..."
      />
      {mutation.isPending && (
        <span class="text-[0.72rem] text-text-muted mt-1 block">Saving...</span>
      )}
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div class="panel-shell animate-pulse">
      <div class="h-7 w-20 bg-surface-3 rounded mb-6" />
      <div class="mb-8">
        <div class="h-6 w-48 bg-surface-3 rounded mb-2" />
        <div class="h-4 w-32 bg-surface-3 rounded mb-2" />
        <div class="h-4 w-64 bg-surface-3 rounded" />
      </div>
      {[1, 2, 3, 4].map((i) => (
        <div key={i} class="mb-6">
          <div class="h-3 w-24 bg-surface-3 rounded mb-2.5" />
          <div class="h-12 w-full bg-surface-3 rounded" />
        </div>
      ))}
    </div>
  )
}

export function DomainDetail({ domainId, onBack }) {
  const { data, isLoading } = useQuery(domainDetailQuery(domainId))

  if (isLoading) return <DetailSkeleton />
  if (!data) {
    return (
      <div class="panel-shell">
        <button
          type="button"
          onClick={onBack}
          class="text-[0.82rem] text-text-dim cursor-pointer py-1.5 px-3 rounded-md border border-border bg-transparent font-sans transition-all hover:bg-surface hover:text-text mb-6"
        >
          {'\u2190 Back'}
        </button>
        <div class="text-text-muted text-center py-12">Domain not found.</div>
      </div>
    )
  }

  const domain = data.domain ?? data
  const deskItems = (data.desk_items ?? []).map(normDeskItem)
  const openCaptures = (data.open_captures ?? []).map(normCapture)
  const recentMemories = (data.recent_memories ?? []).map(normMemory)

  return (
    <div class="panel-shell">
      <button
        type="button"
        onClick={onBack}
        class="text-[0.82rem] text-text-dim cursor-pointer py-1.5 px-3 rounded-md border border-border bg-transparent font-sans transition-all hover:bg-surface hover:text-text mb-6"
      >
        {'\u2190 Back'}
      </button>

      <div class="mb-8">
        <div class="flex items-center gap-3 mb-2">
          <h2 class="panel-title">{domain.name}</h2>
          {domain.importance && <ImportanceDots value={domain.importance} />}
        </div>

        <div class="flex items-center gap-3 flex-wrap">
          <DriftLabel state={data.drift_state} label={data.drift_label} />
          {domain.last_touched_at && (
            <span class="text-[0.78rem] text-text-dim">
              Last touched {formatTouched(domain.last_touched_at)}
            </span>
          )}
        </div>

        {domain.status_line && (
          <div class="text-[0.88rem] text-text-dim mt-2">{domain.status_line}</div>
        )}
      </div>

      <Section
        label="On Desk"
        empty="Nothing on the desk for this domain."
      >
        {deskItems.length > 0 && (
          <div class="flex flex-col gap-1.5">
            {deskItems.map((item) => (
              <div key={item.id} class="surface-card px-4 py-3 flex items-center gap-3">
                <span class="w-2 h-2 rounded-full bg-amber shrink-0" />
                <span class="text-[0.88rem]">{item.title}</span>
                {item.status && (
                  <span class="ml-auto text-[0.72rem] text-text-muted font-mono">
                    {item.status}
                  </span>
                )}
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section
        label="Open"
        empty="No open captures."
      >
        {openCaptures.length > 0 && (
          <div class="flex flex-col gap-1.5">
            {openCaptures.map((cap) => (
              <div key={cap.id} class="surface-card px-4 py-3 flex items-center gap-3">
                <span class="w-2 h-2 rounded-full bg-blue shrink-0" />
                <span class="text-[0.88rem] truncate">{cap.title}</span>
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section
        label="Recent Memory"
        empty="No recent memories."
      >
        {recentMemories.length > 0 && (
          <div class="flex flex-col gap-1.5">
            {recentMemories.map((mem) => (
              <div key={mem.id} class="surface-card px-4 py-3 flex items-center gap-3">
                <span class="w-2 h-2 rounded-full bg-purple shrink-0" />
                <span class="text-[0.88rem] truncate">{mem.title}</span>
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section label="Briefing">
        <BriefingEditor
          domainId={domainId}
          initial={domain.briefing?.Valid ? domain.briefing.String : (domain.briefing ?? "")}
        />
      </Section>
    </div>
  )
}
