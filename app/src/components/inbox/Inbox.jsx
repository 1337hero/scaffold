import { useState } from 'preact/hooks'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { InboxGroup } from './InboxGroup.jsx'
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

  const handleOverride = (item) => {
    const typeInput = window.prompt(
      'Override type (Identity, Goal, Decision, Todo, Idea, Preference, Fact, Event, Observation)',
      'Todo',
    )
    if (!typeInput) return

    const actionInput = window.prompt('Override action (do, explore, reference, waiting)', item.triageAction || 'reference')
    if (!actionInput) return

    const importanceInput = window.prompt('Importance (0.0 to 1.0)', '0.8')
    if (!importanceInput) return
    const importance = Number(importanceInput)
    if (!Number.isFinite(importance) || importance < 0 || importance > 1) {
      setActionError('Importance must be a number between 0 and 1')
      return
    }

    const tagsInput = window.prompt('Tags (comma-separated, optional)', '')
    const tags = (tagsInput || '')
      .split(',')
      .map((tag) => tag.trim())
      .filter(Boolean)

    setActionError('')
    overrideMutation.mutate({
      captureID: item.id,
      payload: {
        type: typeInput.trim(),
        action: actionInput.trim().toLowerCase(),
        importance,
        tags,
      },
    })
  }

  if (isLoading) return null

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
    </div>
  )
}
