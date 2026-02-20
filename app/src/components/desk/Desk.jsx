import { useState, useEffect } from 'preact/hooks'
import { useQuery } from '@tanstack/react-query'
import { TheOne } from './TheOne.jsx'
import { TaskRow } from './TaskRow.jsx'
import { deskQuery } from '@/api/queries.js'

function formatDate() {
  const now = new Date()
  return {
    dayName: now.toLocaleDateString('en-US', { weekday: 'long' }),
    dateFull: now.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' }),
  }
}

function formatTime() {
  const now = new Date()
  const h = now.getHours() % 12 || 12
  const m = now.getMinutes().toString().padStart(2, '0')
  const ampm = now.getHours() >= 12 ? 'PM' : 'AM'
  return `${h}:${m} ${ampm}`
}

export function Desk() {
  const { data, isLoading } = useQuery(deskQuery)
  // Local UI state for interactivity — initialized from query data on first load
  const [theOne, setTheOne] = useState(null)
  const [tasks, setTasks] = useState(null)
  const [clock, setClock] = useState(formatTime)
  const [{ dayName, dateFull }] = useState(formatDate)

  const loaded = !isLoading && data
  // Seed local state once from query — intentionally one-way
  // Use sentinel `false` to distinguish "seeded as null" from "not yet seeded"
  if (loaded && theOne === null) setTheOne(data.theOne ?? false)
  if (loaded && tasks === null) setTasks(data.tasks ?? [])

  useEffect(() => {
    const interval = setInterval(() => setClock(formatTime()), 60000)
    return () => clearInterval(interval)
  }, [])

  if (!loaded || tasks === null) return null

  const hasItems = theOne && tasks
  const allItems = hasItems ? [theOne, ...tasks] : []
  const doneCount = allItems.filter(t => t.done).length
  const totalCount = allItems.length
  const progressPct = totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0

  function toggleTheOne() {
    setTheOne(prev => ({ ...prev, done: !prev.done }))
  }

  function toggleMicroStep(stepId) {
    setTheOne(prev => ({
      ...prev,
      microSteps: prev.microSteps.map(s =>
        s.id === stepId ? { ...s, done: !s.done } : s
      ),
    }))
  }

  function toggleTask(taskId) {
    setTasks(prev => prev.map(t =>
      t.id === taskId ? { ...t, done: !t.done } : t
    ))
  }

  return (
    <div class="panel-shell">
      <div class="flex justify-between items-baseline mb-7">
        <div class="flex items-baseline gap-3">
          <span class="text-[1.5rem] font-bold tracking-[-0.03em]">{dayName}</span>
          <span class="text-[0.85rem] text-text-dim">{dateFull}</span>
        </div>
        <span class="font-mono text-[0.95rem] text-text-muted">{clock}</span>
      </div>

      <div class="flex items-center gap-2 mb-6">
        <div class="flex-1 h-1 bg-surface-2 rounded-[2px] overflow-hidden">
          <div
            class="h-full rounded-[2px] transition-[width] duration-500 motion-reduce:transition-none"
            style={{ width: `${progressPct}%`, background: 'linear-gradient(90deg, var(--color-green), var(--color-amber))' }}
          />
        </div>
        <span class="font-mono text-[0.72rem] text-text-muted">{doneCount} / {totalCount}</span>
      </div>

      {!hasItems ? (
        <div class="bg-surface border border-border rounded-lg p-8 mb-2 text-center">
          <div class="text-text-muted text-[0.9rem] mb-1">No tasks on the desk yet.</div>
          <div class="text-text-dim text-[0.75rem]">The morning prioritize will populate this, or capture something via Signal.</div>
        </div>
      ) : (
        <>
          <TheOne
            task={theOne}
            onToggle={toggleTheOne}
            onToggleStep={toggleMicroStep}
          />

          {tasks.map((task) => (
            <TaskRow key={task.id} task={task} onToggle={() => toggleTask(task.id)} />
          ))}
        </>
      )}

      {(data?.doneToday?.length > 0) && (
        <>
          <div class="flex items-center gap-3 mt-6 mb-3.5">
            <span class="text-[0.68rem] font-semibold uppercase tracking-[0.1em] text-text-muted">Done today</span>
            <span class="flex-1 h-px bg-border" />
          </div>
          <div class="opacity-45">
            {data.doneToday.map((item) => (
              <div key={item.id} class="flex items-center gap-3 py-2">
                <div class="w-[22px] h-[22px] rounded-md border-2 border-green bg-green-dim shrink-0 flex items-center justify-center text-green text-[0.75rem] font-bold">
                  {'\u2713'}
                </div>
                <span class="text-[0.82rem] line-through text-text-muted font-normal">{item.title}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
