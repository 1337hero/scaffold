import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { dashboardQuery, calendarQuery, tasksQuery, completeTask } from "@/api/queries.js"
import TodayHeader from "@/components/dashboard/TodayHeader.jsx"
import TaskList from "@/components/dashboard/TaskList.jsx"
import DoneToday from "@/components/dashboard/DoneToday.jsx"
import CalendarPanel from "@/components/dashboard/CalendarPanel.jsx"
import GoalsOverview from "@/components/dashboard/GoalsOverview.jsx"
import DomainHealth from "@/components/dashboard/DomainHealth.jsx"

const Dashboard = () => {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery(dashboardQuery)
  const { data: events = [] } = useQuery(calendarQuery)
  const { data: tomorrowTasks = [] } = useQuery(tasksQuery(null, null, "pending", "tomorrow"))
  const { data: weekTasks = [] } = useQuery(tasksQuery(null, null, "pending", "week"))

  const complete = useMutation({
    mutationFn: completeTask,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dashboard"] })
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
    },
  })

  if (isLoading) {
    return (
      <div class="flex items-center justify-center min-h-[60vh]">
        <div class="w-6 h-6 border-2 border-text/20 border-t-text/60 rounded-full animate-spin" />
      </div>
    )
  }

  const todaysTasks = data?.TodaysTasks ?? []
  const overdueTasks = data?.OverdueTasks ?? []
  const goalsWithProgress = data?.GoalsWithProgress ?? []
  const domains = data?.DomainHealth ?? []
  const doneToday = data?.DoneToday ?? []

  return (
    <div class="space-y-10">
      <TodayHeader />

      <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
        <div class="lg:col-span-8 space-y-10">
          <GoalsOverview goals={goalsWithProgress} />
          <TaskList
            tasks={todaysTasks}
            overdueTasks={overdueTasks}
            tomorrowTasks={tomorrowTasks}
            weekTasks={weekTasks}
            onComplete={(id) => complete.mutate(id)}
          />
          <DoneToday tasks={doneToday} />
        </div>

        <div class="lg:col-span-4 space-y-10">
          <CalendarPanel events={events} />
          <DomainHealth domains={domains} />
        </div>
      </div>
    </div>
  )
}

export default Dashboard
