import { useState } from 'preact/hooks'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { InboxGroup } from './InboxGroup.jsx'
import { OverrideModal } from './OverrideModal.jsx'
import {
  archiveInboxCapture,
  confirmInboxCapture,
  inboxQuery,
  overrideInboxCapture,
} from '@/api/queries.js'

const views = ['By Action', 'By Time', 'By Type']

export function Inbox() {
  const queryClient = useQueryClient()
  const { data: groups = [], isLoading } = useQuery(inboxQuery)
  const [activeView, setActiveView] = useState('By Action')
  const [actionError, setActionError] = useState('')
  const [overrideItem, setOverrideItem] = useState(null)

  const invalidateInbox = () => {
    queryClient.invalidateQueries({ queryKey: ['inbox'] })
    queryClient.invalidateQueries({ queryKey: ['memories'] })
  }

  const confirmMutation = useMutation({
    mutationFn: (captureID) => confirmInboxCapture(captureID),
    onSuccess: invalidateInbox,
    onError: (error) => setActionError(error?.message ?? 'Failed to confirm item'),
  })

  const archiveMutation = useMutation({
    mutationFn: (captureID) => archiveInboxCapture(captureID),
    onSuccess: invalidateInbox,
    onError: (error) => setActionError(error?.message ?? 'Failed to archive item'),
  })

  const overrideMutation = useMutation({
    mutationFn: ({ captureID, payload }) => overrideInboxCapture(captureID, payload),
    onSuccess: invalidateInbox,
    onError: (error) => setActionError(error?.message ?? 'Failed to override classification'),
  })

  const actionPending = confirmMutation.isPending || archiveMutation.isPending || overrideMutation.isPending

  const handleConfirm = (item) => {
    setActionError('')
    confirmMutation.mutate(item.id)
  }

  const handleArchive = (item) => {
    setActionError('')
    archiveMutation.mutate(item.id)
  }

  const handleOverride = (item) => setOverrideItem(item)

  const handleOverrideConfirm = (overrides) => {
    setActionError('')
    overrideMutation.mutate(
      { captureID: overrideItem.id, payload: overrides },
      { onSettled: () => setOverrideItem(null) },
    )
  }

  if (isLoading) return (
    <div class="panel-shell">
      <div class="flex justify-between items-center mb-8">
        <div class="h-7 w-24 bg-surface-2 rounded-md animate-pulse" />
        <div class="flex gap-1.5">
          {[1, 2, 3].map((i) => <div key={i} class="h-9 w-20 bg-surface-2 rounded-md animate-pulse" />)}
        </div>
      </div>
      {[1, 2, 3].map((g) => (
        <div key={g} class="mb-8">
          <div class="flex items-center gap-3 mb-3.5">
            <div class="w-2.5 h-2.5 rounded-full bg-surface-3 animate-pulse" />
            <div class="h-5 w-32 bg-surface-2 rounded-md animate-pulse" />
          </div>
          {[1, 2].map((c) => (
            <div key={c} class="surface-card py-5 px-6 mb-2 flex items-start gap-4">
              <div class="h-5 w-14 bg-surface-2 rounded-sm animate-pulse shrink-0" />
              <div class="flex-1 space-y-2.5">
                <div class="h-5 w-3/4 bg-surface-2 rounded-md animate-pulse" />
                <div class="h-4 w-1/2 bg-surface-2 rounded-md animate-pulse" />
              </div>
              <div class="h-4 w-16 bg-surface-2 rounded-md animate-pulse shrink-0" />
            </div>
          ))}
        </div>
      ))}
    </div>
  )

  return (
    <div class="panel-shell">
      <div class="flex justify-between items-center mb-8">
        <h2 class="panel-title">Inbox</h2>
        <div class="flex gap-1.5">
          {views.map((v) => (
            <button
              type="button"
              key={v}
              onClick={() => setActiveView(v)}
              class={`text-[0.86rem] font-medium py-2 px-4 rounded-md border font-sans cursor-pointer transition-all
                ${activeView === v
                  ? 'bg-surface-2 text-text border-border-light'
                  : 'bg-transparent text-text-dim border-border hover:bg-surface-2 hover:text-text hover:border-border-light'
                }`}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      {actionError && (
        <div class="mb-5 rounded-md border border-red-500/40 bg-red-500/10 px-4 py-3 text-[0.9rem] text-red-300">
          {actionError}
        </div>
      )}

      {activeView === 'By Action' ? (
        groups.map((group) => (
          <InboxGroup
            key={group.id}
            group={group}
            onConfirm={handleConfirm}
            onOverride={handleOverride}
            onArchive={handleArchive}
            actionPending={actionPending}
          />
        ))
      ) : (
        <div class="text-text-muted text-sm py-12 text-center">
          {activeView} view coming soon
        </div>
      )}

      <OverrideModal
        item={overrideItem}
        onConfirm={handleOverrideConfirm}
        onClose={() => setOverrideItem(null)}
      />
    </div>
  )
}
