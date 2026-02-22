import { useState, useEffect } from 'preact/hooks'
import { useQuery } from '@tanstack/react-query'
import { TheOne } from './TheOne.jsx'
import { TaskRow } from './TaskRow.jsx'
import { CalendarEvents } from './CalendarEvents.jsx'
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

function SkeletonPulse({ className }) {
  return <div class={`animate-pulse bg-surface-card rounded ${className}`} />
}

function DeskSkeleton({ dayName, dateFull, clock }) {
  return (
    <div class="panel-shell">
      <div class="flex justify-between items-baseline mb-9">
        <div class="flex items-baseline gap-3.5">
          <span class="text-[2rem] font-bold tracking-[-0.04em]">{dayName}</span>
          <span class="text-[1rem] text-text-dim">{dateFull}</span>
        </div>
        <span class="font-mono text-[1rem] text-text-muted">{clock}</span>
      </div>
      <div class="flex items-center gap-2.5 mb-8">
        <div class="flex-1 h-1.5 bg-surface-2 rounded-[3px]" />
        <span class="font-mono text-[0.85rem] text-text-muted">- / -</span>
      </div>
      <SkeletonPulse className="h-[140px] mb-3 rounded-xl" />
      <SkeletonPulse className="h-[72px] mb-2" />
      <SkeletonPulse className="h-[72px] mb-2" />
    </div>
  )
}

export function Desk() {
  const { data, isLoading } = useQuery(deskQuery)
  const [doneIds, setDoneIds] = useState(new Set())
  const [stepDoneIds, setStepDoneIds] = useState(new Set())
  const [clock, setClock] = useState(formatTime)
  const { dayName, dateFull } = formatDate()

  useEffect(() => {
    const interval = setInterval(() => setClock(formatTime()), 60000)
    return () => clearInterval(interval)
  }, [])

  if (isLoading || !data) {
    return <DeskSkeleton dayName={dayName} dateFull={dateFull} clock={clock} />
  }

  const toggleDone = (id) => setDoneIds(prev => {
    const next = new Set(prev)
    next.has(id) ? next.delete(id) : next.add(id)
    return next
  })

  const toggleStepDone = (stepId) => setStepDoneIds(prev => {
    const next = new Set(prev)
    next.has(stepId) ? next.delete(stepId) : next.add(stepId)
    return next
  })

  const effectiveDone = (item) => !!(item.done ^ doneIds.has(item.id))

  const applyDone = (item) => ({
    ...item,
    done: effectiveDone(item),
    microSteps: item.microSteps?.map(s => ({
      ...s,
      done: !!(s.done ^ stepDoneIds.has(s.id)),
    })),
  })

  const { theOne, tasks = [] } = data
  const theOneEffective = theOne ? applyDone(theOne) : null
  const tasksEffective = tasks.map(applyDone)

  const allItems = theOneEffective ? [theOneEffective, ...tasksEffective] : tasksEffective
  const doneCount = allItems.filter(t => t.done).length
  const totalCount = allItems.length
  const progressPct = totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0

  return (
    <div class="panel-shell">
      <div class="flex justify-between items-baseline mb-9">
        <div class="flex items-baseline gap-3.5">
          <span class="text-[2rem] font-bold tracking-[-0.04em]">{dayName}</span>
          <span class="text-[1rem] text-text-dim">{dateFull}</span>
        </div>
        <span class="font-mono text-[1rem] text-text-muted">{clock}</span>
      </div>

      <CalendarEvents />

      <div class="flex items-center gap-2.5 mb-8">
        <div class="flex-1 h-1.5 bg-surface-2 rounded-[3px] overflow-hidden">
          <div
            class="h-full rounded-[3px] transition-[width] duration-500 motion-reduce:transition-none"
            style={{ width: `${progressPct}%`, background: 'linear-gradient(90deg, var(--color-green), var(--color-amber))' }}
          />
        </div>
        <span class="font-mono text-[0.85rem] text-text-muted">{doneCount} / {totalCount}</span>
      </div>

      {totalCount === 0 ? (
        <div class="bg-surface border border-border rounded-lg p-9 mb-2 text-center">
          <div class="text-text-muted text-[1rem] mb-1">No tasks on the desk yet.</div>
          <div class="text-text-dim text-[0.9rem]">The morning prioritize will populate this, or capture something via Signal.</div>
        </div>
      ) : (
        <>
          {theOneEffective && (
            <TheOne
              task={theOneEffective}
              onToggle={() => toggleDone(theOne.id)}
              onToggleStep={(stepId) => toggleStepDone(stepId)}
            />
          )}

          {tasksEffective.map((task) => (
            <TaskRow key={task.id} task={task} onToggle={() => toggleDone(task.id)} />
          ))}
        </>
      )}

      {(data?.doneToday?.length > 0) && (
        <>
          <div class="flex items-center gap-3.5 mt-8 mb-4">
            <span class="text-[0.78rem] font-semibold uppercase tracking-[0.1em] text-text-muted">Done today</span>
            <span class="flex-1 h-px bg-border" />
          </div>
          <div class="opacity-45">
            {data.doneToday.map((item) => (
              <div key={item.id} class="flex items-center gap-3.5 py-2.5">
                <div class="w-[24px] h-[24px] rounded-md border-2 border-green bg-green-dim shrink-0 flex items-center justify-center text-green text-[0.82rem] font-bold">
                  {'\u2713'}
                </div>
                <span class="text-[0.94rem] line-through text-text-muted font-normal">{item.title}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
