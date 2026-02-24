import { useState } from "preact/hooks"
import {
  completeTask,
  deleteGoal,
  deleteNote,
  deleteTask,
  domainHealthQuery,
  domainsQuery,
  goalsQuery,
  notesQuery,
  tasksQuery,
  updateGoal,
  updateNote,
  updateTask,
} from "@/api/queries.js"
import GoalCard from "@/components/notebooks/GoalCard.jsx"
import InlineCreate from "@/components/notebooks/InlineCreate.jsx"
import NoteItem from "@/components/notebooks/NoteItem.jsx"
import TaskItem from "@/components/notebooks/TaskItem.jsx"
import { nullable } from "@/utils/nullable.js"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

function domainColor(domain) {
  if (domain?.Color?.Valid) return domain.Color.String
  const colors = {
    "Work/Business": "#5B8DB8",
    "Personal Projects": "#8B6BB1",
    Homelife: "#C47D3A",
    "Personal Development": "#5A9E6F",
    Relationships: "#C4617A",
    Finances: "#3D9E9E",
    Hobbies: "#C4663A",
  }
  return colors[domain?.Name] || "#9C8E7A"
}

const ChevronLeftIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="m15 18-6-6 6-6" />
  </svg>
)

const BookOpenIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
    <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
  </svg>
)

const DRIFT_COLORS = {
  active: "#27ae60",
  drifting: "#f39c12",
  neglected: "#e74c3c",
  cold: "#95a5a6",
  overactive: "#3498db",
}

const Notebook = ({ domainId, onBack }) => {
  const queryClient = useQueryClient()
  const { data: domains, isLoading: domainsLoading } = useQuery(domainsQuery)
  const { data: healthData = [] } = useQuery(domainHealthQuery)
  const { data: goals = [] } = useQuery(goalsQuery(domainId))
  const { data: tasks = [] } = useQuery(tasksQuery(domainId))
  const { data: notes = [] } = useQuery(notesQuery(domainId))

  const completeMutation = useMutation({
    mutationFn: completeTask,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["goals"] })
    },
  })

  const saveMutation = useMutation({
    mutationFn: ({ id, data }) => updateNote(id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["notes"] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteNote,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["notes"] }),
  })

  const updateTaskMutation = useMutation({
    mutationFn: ({ id, data }) => updateTask(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["goals"] })
    },
  })

  const deleteTaskMutation = useMutation({
    mutationFn: deleteTask,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["tasks"] }),
  })

  const updateGoalMutation = useMutation({
    mutationFn: ({ id, data }) => updateGoal(id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["goals"] }),
  })

  const deleteGoalMutation = useMutation({
    mutationFn: deleteGoal,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["goals"] })
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
    },
  })

  if (domainsLoading) {
    return (
      <div class="space-y-10 animate-pulse">
        <div class="h-12 w-48 bg-black/5 rounded-2xl" />
        <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div class="lg:col-span-8 space-y-4">
            {[1, 2, 3].map((i) => <div key={i} class="h-20 bg-black/5 rounded-3xl" />)}
          </div>
          <div class="lg:col-span-4">
            <div class="h-48 bg-black/5 rounded-3xl" />
          </div>
        </div>
      </div>
    )
  }

  const domain = (domains || []).find((d) => String(d.ID) === String(domainId))

  if (!domain) {
    return (
      <div class="space-y-6">
        <button
          type="button"
          onClick={onBack}
          class="p-3 bg-[var(--color-card-bg)] border border-app-border rounded-2xl hover:bg-app-ink hover:text-white transition-all card-shadow"
        >
          <ChevronLeftIcon />
        </button>
        <p class="text-app-muted text-center py-12">Domain not found.</p>
      </div>
    )
  }

  const color = domainColor(domain)
  const health = healthData.find((h) => h.ID === domain.ID)
  const healthScore = health?.HealthScore ?? 0
  const driftState = (health?.State || "active").toLowerCase()
  const driftColor = DRIFT_COLORS[driftState] || DRIFT_COLORS.active

  const [taskTab, setTaskTab] = useState("open")
  const recurringTasks = tasks.filter((t) => t.Recurring?.Valid && t.Recurring.String)
  const openTasks = tasks.filter((t) => t.Status !== "done")
  const doneTasks = tasks.filter((t) => t.Status === "done")
  const completionRate =
    tasks.length > 0 ? Math.round((doneTasks.length / tasks.length) * 100) : 0

  const TASK_TABS = [
    { key: "open", label: "Open" },
    { key: "done", label: "Done" },
    { key: "recurring", label: "Recurring" },
  ]
  const visibleTasks = taskTab === "open" ? openTasks
    : taskTab === "done" ? doneTasks
    : recurringTasks

  return (
    <div class="space-y-10">
      <header class="flex items-center gap-6">
        <button
          type="button"
          onClick={onBack}
          class="p-3 bg-[var(--color-card-bg)] border border-app-border rounded-2xl hover:bg-app-ink hover:text-white transition-all card-shadow"
        >
          <ChevronLeftIcon />
        </button>
        <div class="flex items-center gap-4">
          <div
            class="w-12 h-12 rounded-2xl flex items-center justify-center text-white shadow-lg"
            style={{ backgroundColor: color }}
          >
            <BookOpenIcon />
          </div>
          <div>
            <h2 class="text-3xl font-serif italic font-semibold">{domain.Name}</h2>
            <div class="flex items-center gap-2 mt-1">
              <span class="text-[10px] mono uppercase opacity-40">{driftState}</span>
              <div class="w-1 h-1 rounded-full bg-app-border" />
              <span class="text-[10px] mono uppercase opacity-40">
                {Math.round(healthScore * 100)}% Health
              </span>
            </div>
          </div>
        </div>
      </header>

      <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
        <div class="lg:col-span-8 space-y-12">
          {/* Goals */}
          <section class="space-y-6">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">
                Active Goals
              </h3>
              <InlineCreate domainId={domainId} type="goal" compact />
            </div>
            {goals.length === 0 ? (
              <div class="py-12 text-center border-2 border-dashed border-app-border rounded-3xl opacity-30">
                <p class="font-serif italic">No goals yet.</p>
              </div>
            ) : (
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                {goals.map((goal) => (
                  <GoalCard
                    key={goal.ID}
                    goal={goal}
                    domain={domain}
                    color={color}
                    domains={domains || []}
                    tasks={tasks}
                    onSave={(id, data) => updateGoalMutation.mutate({ id, data })}
                    onDelete={(id) => deleteGoalMutation.mutate(id)}
                  />
                ))}
              </div>
            )}
          </section>

          {/* Tasks */}
          <section class="space-y-6">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-4">
                <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">Tasks</h3>
                <div class="flex gap-1">
                  {TASK_TABS.map((t) => (
                    <button
                      key={t.key}
                      type="button"
                      onClick={() => setTaskTab(t.key)}
                      class={`px-2.5 py-1 text-[10px] mono uppercase tracking-wide rounded-full transition-colors cursor-pointer ${
                        taskTab === t.key
                          ? "bg-app-ink/8 text-app-ink font-semibold"
                          : "text-app-muted hover:text-app-ink hover:bg-app-ink/4"
                      }`}
                    >
                      {t.label}
                    </button>
                  ))}
                </div>
              </div>
              <InlineCreate domainId={domainId} type="task" compact />
            </div>
            {visibleTasks.length === 0 ? (
              <div class="py-12 text-center border-2 border-dashed border-app-border rounded-3xl opacity-30">
                <p class="font-serif italic">
                  {taskTab === "open" ? "No open tasks." : taskTab === "done" ? "No completed tasks." : "No recurring tasks."}
                </p>
              </div>
            ) : (
              <div class="bg-white rounded-3xl border border-app-border card-shadow divide-y divide-app-border">
                {visibleTasks.map((task) => (
                  <TaskItem
                    key={task.ID}
                    task={task}
                    onComplete={taskTab !== "done" ? () => completeMutation.mutate(task.ID) : undefined}
                    domains={domains || []}
                    goals={goals}
                    onSave={(id, data) => updateTaskMutation.mutate({ id, data })}
                    onDelete={(id) => deleteTaskMutation.mutate(id)}
                  />
                ))}
              </div>
            )}
          </section>

          {/* Notes */}
          <section class="space-y-6">
            <div class="flex items-center justify-between">
              <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40">Notes</h3>
              <InlineCreate domainId={domainId} type="note" compact />
            </div>
            {notes.length === 0 ? (
              <div class="py-12 text-center border-2 border-dashed border-app-border rounded-3xl opacity-30">
                <p class="font-serif italic">No notes yet.</p>
              </div>
            ) : (
              <div class="grid grid-cols-1 gap-4">
                {notes.map((note) => (
                  <NoteItem
                    key={note.ID}
                    note={note}
                    onSave={(id, data) => saveMutation.mutate({ id, data })}
                    onDelete={(id) => deleteMutation.mutate(id)}
                  />
                ))}
              </div>
            )}
          </section>
        </div>

        {/* Stats sidebar */}
        <div class="lg:col-span-4 space-y-8">
          <div class="p-6 bg-app-ink text-[#F5F0E8] rounded-3xl shadow-xl">
            <h3 class="text-xs font-bold mono uppercase tracking-widest opacity-40 mb-4">
              Domain Stats
            </h3>
            <div class="space-y-4">
              <div class="flex justify-between items-end">
                <span class="text-sm opacity-60">Completion Rate</span>
                <span class="text-xl font-bold mono">{completionRate}%</span>
              </div>
              <div class="flex justify-between items-end">
                <span class="text-sm opacity-60">Active Goals</span>
                <span class="text-xl font-bold mono">{goals.length}</span>
              </div>
              <div class="flex justify-between items-end">
                <span class="text-sm opacity-60">Open Tasks</span>
                <span class="text-xl font-bold mono">{openTasks.length}</span>
              </div>
              <div class="flex justify-between items-end">
                <span class="text-sm opacity-60">Notes</span>
                <span class="text-xl font-bold mono">{notes.length}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Notebook
