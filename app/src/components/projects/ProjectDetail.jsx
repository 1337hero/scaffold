import { archiveProject, patchProject, projectDetailQuery, projectTasksQuery } from "@/api/queries.js";
import ActivityLog from "@/components/projects/ActivityLog.jsx";
import ChecklistCard from "@/components/projects/ChecklistCard.jsx";
import MilestoneList from "@/components/projects/MilestoneList.jsx";
import ProjectForm from "@/components/projects/ProjectForm.jsx";
import { daysSince } from "@/lib/utils.js";
import { RiAlarmWarningLine, RiCheckLine, RiPencilLine, RiArchiveLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const typeLabels = { project: "Project", area: "Area", retainer: "Retainer" };

function resetThisMonth(lastResetAt) {
  return Boolean(lastResetAt && lastResetAt.slice(0, 7) === new Date().toISOString().slice(0, 7));
}

const ProjectDetail = ({ projectId, domains }) => {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);

  const { data, isLoading, error } = useQuery(projectDetailQuery(projectId));
  const { data: tasks = [] } = useQuery(projectTasksQuery(projectId));

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["project-detail", projectId] });
    queryClient.invalidateQueries({ queryKey: ["projects-list"] });
  };
  const editMutation = useMutation({
    mutationFn: (updates) => patchProject(projectId, updates),
    onSuccess: () => {
      setEditing(false);
      invalidate();
    },
  });
  const archiveMutation = useMutation({ mutationFn: () => archiveProject(projectId), onSuccess: invalidate });

  if (isLoading) return <p class="text-app-muted">Loading project…</p>;
  if (error) return <p class="text-status-error">Couldn't load project: {error.message}</p>;

  const { project, milestones, milestoneCompleted, milestoneTotal, checklists } = data;
  const slippingDays = project.lastActivityAt ? daysSince(project.lastActivityAt) : null;
  const isSlipping =
    project.status === "active" && project.type !== "retainer" &&
    (slippingDays === null || slippingDays >= (project.type === "area" ? 14 : 7));

  if (editing) {
    return (
      <ProjectForm
        project={project}
        domains={domains}
        submitting={editMutation.isPending}
        onSubmit={(updates) => editMutation.mutate(updates)}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <div class="space-y-4">
      <div class="flex items-start justify-between gap-3 flex-wrap">
        <div>
          <div class="flex items-center gap-2 flex-wrap">
            <h1 class="font-serif italic text-2xl font-semibold tracking-tight">{project.name}</h1>
            <span class="px-2 py-0.5 rounded-full bg-app-border text-[10px] font-bold uppercase">
              {typeLabels[project.type]}
            </span>
            {project.status !== "active" && (
              <span class="px-2 py-0.5 rounded-full border border-app-border text-[10px] uppercase text-app-muted">
                {project.status.replace("_", " ")}
              </span>
            )}
            {isSlipping && (
              <a
                href="#/today"
                class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-status-warning/15 text-status-warning text-[10px] font-bold uppercase"
                title="See slipping on Today"
              >
                <RiAlarmWarningLine size={12} />
                {slippingDays === null ? "never touched" : `${slippingDays}d quiet`}
              </a>
            )}
            {project.type === "retainer" && (
              <span class="flex items-center gap-1 px-2 py-0.5 rounded-full border border-app-border text-[10px] uppercase text-app-muted">
                {resetThisMonth(project.lastResetAt) && <RiCheckLine size={12} class="text-status-success" />}
                {resetThisMonth(project.lastResetAt) ? "reset this month" : "reset pending"}
              </span>
            )}
          </div>
          <p class="text-sm text-app-muted mt-1">
            {[
              project.startDate && `started ${project.startDate}`,
              project.endDate && `ends ${project.endDate}`,
              project.domainId != null && domains.find((d) => d.ID === project.domainId)?.Name,
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
          {project.description && <p class="text-sm mt-2 max-w-prose">{project.description}</p>}
        </div>
        <div class="flex gap-2 shrink-0">
          <button
            type="button"
            onClick={() => setEditing(true)}
            class="flex items-center gap-1 px-3 py-1.5 rounded-full border border-app-border text-xs hover:bg-app-bg"
          >
            <RiPencilLine size={13} /> Edit
          </button>
          <button
            type="button"
            onClick={() => archiveMutation.mutate()}
            class="flex items-center gap-1 px-3 py-1.5 rounded-full border border-app-border text-xs text-status-error hover:bg-status-error/10"
          >
            <RiArchiveLine size={13} /> Archive
          </button>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <MilestoneList
          projectId={projectId}
          milestones={milestones}
          completed={milestoneCompleted}
          total={milestoneTotal}
        />
        <ChecklistCard projectId={projectId} checklists={checklists} />
      </div>

      {tasks.length > 0 && (
        <section aria-label="Tasks" class="p-5 rounded-xl bg-card-bg border border-app-border">
          <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3">Tasks</h2>
          <ul class="space-y-1">
            {tasks.map((t) => (
              <li key={t.id}>
                <a href="#/tasks" class="flex items-baseline justify-between gap-3 py-1 px-1 rounded hover:bg-app-bg text-sm">
                  <span>{t.title}</span>
                  {t.dueDate && <span class="text-xs text-app-muted shrink-0">due {t.dueDate}</span>}
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}

      <ActivityLog projectId={projectId} isRetainer={project.type === "retainer"} />
    </div>
  );
};

export default ProjectDetail;
