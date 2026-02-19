import { useState, useEffect } from 'preact/hooks'
import { useQuery } from '@tanstack/react-query'
import { TheOne } from './TheOne.jsx'
import { TaskRow } from './TaskRow.jsx'
import { deskQuery } from '@/api/queries.js'
import { deskData } from '@/data/mock.js'

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

  // Seed local state once from query — intentionally one-way
  if (data && theOne === null) setTheOne(data.theOne)
  if (data && tasks === null) setTasks(data.tasks)

  useEffect(() => {
    const interval = setInterval(() => setClock(formatTime()), 60000)
    return () => clearInterval(interval)
  }, [])

  if (isLoading || !theOne || !tasks) return null

  const doneCount = [theOne, ...tasks].filter(t => t.done).length
  const totalCount = 1 + tasks.length
  const progressPct = Math.round((doneCount / totalCount) * 100)

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
    <div class="max-w-[760px] mx-auto px-8 py-8 pb-[100px] max-md:px-4 max-md:py-5">
      <div class="flex justify-between items-baseline mb-7">
        <div class="flex items-baseline gap-3">
          <span class="text-2xl font-bold tracking-tight">{dayName}</span>
          <span class="text-[0.85rem] text-text-dim">{dateFull}</span>
        </div>
        <span class="font-mono text-[0.95rem] text-text-muted">{clock}</span>
      </div>

      <div class="flex items-center gap-2 mb-6">
        <div class="flex-1 h-1 bg-surface-2 rounded-sm overflow-hidden">
          <div
            class="h-full rounded-sm transition-[width] duration-500 motion-reduce:transition-none"
            style={{ width: `${progressPct}%`, background: 'linear-gradient(90deg, var(--color-green), var(--color-amber))' }}
          />
        </div>
        <span class="font-mono text-[0.72rem] text-text-muted">{doneCount} / {totalCount}</span>
      </div>

      <TheOne
        task={theOne}
        onToggle={toggleTheOne}
        onToggleStep={toggleMicroStep}
      />

      {tasks.map((task) => (
        <TaskRow key={task.id} task={task} onToggle={() => toggleTask(task.id)} />
      ))}

      {deskData.doneToday.length > 0 && (
        <>
          <div class="flex items-center gap-3 mt-6 mb-3.5">
            <span class="text-[0.68rem] font-semibold uppercase tracking-[0.1em] text-text-muted">Done today</span>
            <span class="flex-1 h-px bg-border" />
          </div>
          <div class="opacity-45">
            {deskData.doneToday.map((item) => (
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
