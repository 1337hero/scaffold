import {
  completeTask,
  createTask,
  deleteTask,
  domainsQuery,
  projectsListQuery,
  setTop3,
  tasksListQuery,
  top3IdsQuery,
  updateTask,
} from "@/api/queries.js";
import TaskFilters from "@/components/tasks/TaskFilters.jsx";
import TaskForm from "@/components/tasks/TaskForm.jsx";
import TaskRow from "@/components/tasks/TaskRow.jsx";
import { useSurface } from "@/hooks/useSurface.jsx";
import { RiAddLine, RiArrowDownSLine, RiArrowUpSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const priorityRank = { urgent: 0, high: 1, normal: 2, low: 3 };

const columns = [
  { key: "title", label: "Task" },
  { key: "project", label: "Project / Area" },
  { key: "dueDate", label: "Due" },
  { key: "priority", label: "Priority" },
];

function applyFilters(tasks, filters, today, weekEnd) {
  return tasks.filter((t) => {
    if (filters.projectId && t.projectId !== filters.projectId) return false;
    if (filters.domainId && String(t.domainId ?? "") !== filters.domainId) return false;
    if (filters.priority && t.priority !== filters.priority) return false;
    if (filters.due === "overdue" && !(t.dueDate && t.dueDate < today)) return false;
    if (filters.due === "today" && t.dueDate !== today) return false;
    if (filters.due === "week" && !(t.dueDate && t.dueDate >= today && t.dueDate <= weekEnd)) return false;
    return true;
  });
}

function compareBy(sort, projectById) {
  const dir = sort.desc ? -1 : 1;
  return (a, b) => {
    let av;
    let bv;
    switch (sort.key) {
      case "priority":
        av = priorityRank[a.priority] ?? 2;
        bv = priorityRank[b.priority] ?? 2;
        break;
      case "project":
        av = projectById[a.projectId]?.name ?? a.domainName ?? "";
        bv = projectById[b.projectId]?.name ?? b.domainName ?? "";
        break;
      case "title":
        av = a.title.toLowerCase();
        bv = b.title.toLowerCase();
        break;
      default:
        // Due date: empty dates sort last regardless of direction.
        av = a.dueDate ?? (sort.desc ? "" : "~");
        bv = b.dueDate ?? (sort.desc ? "" : "~");
    }
    if (av < bv) return -dir;
    if (av > bv) return dir;
    return 0;
  };
}

const Tasks = () => {
  const { surface } = useSurface();
  const queryClient = useQueryClient();

  const [filters, setFilters] = useState({ status: "pending", projectId: "", domainId: "", due: "", priority: "" });
  const [sort, setSort] = useState({ key: "dueDate", desc: false });
  const [expandedId, setExpandedId] = useState(null);
  const [formTask, setFormTask] = useState(null); // null=closed, "new"=create, task=edit

  const { data: tasks = [], isLoading, error } = useQuery(tasksListQuery(surface, filters.status));
  const { data: projects = [] } = useQuery(projectsListQuery());
  const { data: domains = [] } = useQuery(domainsQuery);
  const { data: top3Ids = [] } = useQuery(top3IdsQuery);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["tasks-list"] });
    queryClient.invalidateQueries({ queryKey: ["top3-ids"] });
    queryClient.invalidateQueries({ queryKey: ["today"] });
  };

  const starMutation = useMutation({ mutationFn: setTop3, onSuccess: invalidate });
  const completeMutation = useMutation({ mutationFn: completeTask, onSuccess: invalidate });
  const deleteMutation = useMutation({ mutationFn: deleteTask, onSuccess: invalidate });
  const saveMutation = useMutation({
    mutationFn: ({ id, data }) => (id ? updateTask(id, data) : createTask(data)),
    onSuccess: () => {
      invalidate();
      setFormTask(null);
    },
  });

  const projectById = Object.fromEntries(projects.map((p) => [p.id, p]));
  const today = new Date().toISOString().slice(0, 10);
  const weekEnd = new Date(Date.now() + 7 * 86_400_000).toISOString().slice(0, 10);
  const visible = applyFilters(tasks, filters, today, weekEnd).sort(compareBy(sort, projectById));

  const handleSort = (key) => setSort((s) => ({ key, desc: s.key === key && !s.desc }));

  const handleStar = (task) => {
    const starred = top3Ids.includes(task.id);
    starMutation.mutate(starred ? top3Ids.filter((id) => id !== task.id) : [...top3Ids, task.id]);
  };

  if (error) return <p class="text-status-error">Couldn't load tasks: {error.message}</p>;

  return (
    <div class="space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <h1 class="font-serif italic text-3xl font-semibold tracking-tight">Tasks</h1>
        <button
          type="button"
          onClick={() => setFormTask(formTask === "new" ? null : "new")}
          class="flex items-center gap-1.5 px-4 py-2 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold"
        >
          <RiAddLine size={16} /> New task
        </button>
      </div>

      {formTask === "new" && (
        <TaskForm
          projects={projects}
          domains={domains}
          surface={surface}
          submitting={saveMutation.isPending}
          onSubmit={(data) => saveMutation.mutate({ data })}
          onCancel={() => setFormTask(null)}
        />
      )}

      <TaskFilters filters={filters} onChange={setFilters} projects={projects} domains={domains} />

      {isLoading ? (
        <p class="text-app-muted">Loading tasks…</p>
      ) : visible.length === 0 ? (
        <p class="text-app-muted">No tasks match. Clean slate or narrow filters.</p>
      ) : (
        <div class="overflow-x-auto rounded-xl border border-app-border bg-card-bg/30">
          <table class="w-full text-left">
            <thead>
              <tr class="border-b border-app-border text-xs uppercase tracking-wide text-app-muted">
                <th class="py-2 px-2 w-8" />
                {columns.map((c) => (
                  <th key={c.key} class="py-2 px-2">
                    <button type="button" onClick={() => handleSort(c.key)} class="flex items-center gap-1 uppercase">
                      {c.label}
                      {sort.key === c.key &&
                        (sort.desc ? <RiArrowDownSLine size={14} /> : <RiArrowUpSLine size={14} />)}
                    </button>
                  </th>
                ))}
                <th class="py-2 px-2 w-20" />
              </tr>
            </thead>
            <tbody>
              {visible.map((task) =>
                formTask && formTask !== "new" && formTask.id === task.id ? (
                  <tr key={task.id}>
                    <td colSpan={6} class="py-2">
                      <TaskForm
                        task={task}
                        projects={projects}
                        domains={domains}
                        surface={task.surface}
                        submitting={saveMutation.isPending}
                        onSubmit={(data) => saveMutation.mutate({ id: task.id, data })}
                        onCancel={() => setFormTask(null)}
                      />
                    </td>
                  </tr>
                ) : (
                  <TaskRow
                    key={task.id}
                    task={task}
                    project={projectById[task.projectId]}
                    starred={top3Ids.includes(task.id)}
                    top3Full={top3Ids.length >= 3}
                    expanded={expandedId === task.id}
                    onToggleExpand={() => setExpandedId(expandedId === task.id ? null : task.id)}
                    onStar={() => handleStar(task)}
                    onComplete={() => completeMutation.mutate(task.id)}
                    onEdit={() => setFormTask(task)}
                    onDelete={() => deleteMutation.mutate(task.id)}
                  />
                ),
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default Tasks;
