import { cloneChecklist, createProject, domainsQuery, projectsListQuery } from "@/api/queries.js";
import ProjectDetail from "@/components/projects/ProjectDetail.jsx";
import ProjectForm from "@/components/projects/ProjectForm.jsx";
import ProjectSidebar from "@/components/projects/ProjectSidebar.jsx";
import { useSurface } from "@/hooks/useSurface.jsx";
import { navigate } from "@/hooks/useRoute.js";
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const Projects = ({ projectId }) => {
  const { surface } = useSurface();
  const queryClient = useQueryClient();
  const [creating, setCreating] = useState(false);

  const { data: projects = [], isLoading, error } = useQuery(projectsListQuery(surface));
  const { data: domains = [] } = useQuery(domainsQuery);

  const createMutation = useMutation({
    mutationFn: async ({ data, templateIds }) => {
      const created = await createProject(data);
      for (const templateId of templateIds) {
        await cloneChecklist(created.ID, templateId);
      }
      return created;
    },
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["projects-list"] });
      setCreating(false);
      navigate(`/projects/${created.ID}`);
    },
  });

  if (error) return <p class="text-status-error">Couldn't load projects: {error.message}</p>;

  return (
    <div class="space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <h1 class="font-serif italic text-3xl font-semibold tracking-tight">Projects</h1>
        <button
          type="button"
          onClick={() => setCreating(!creating)}
          class="flex items-center gap-1.5 px-4 py-2 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold"
        >
          <RiAddLine size={16} /> New
        </button>
      </div>

      {creating && (
        <ProjectForm
          domains={domains}
          surface={surface}
          submitting={createMutation.isPending}
          onSubmit={(data, templateIds) => createMutation.mutate({ data, templateIds })}
          onCancel={() => setCreating(false)}
        />
      )}

      {isLoading ? (
        <p class="text-app-muted">Loading projects…</p>
      ) : (
        <div class="grid gap-6 lg:grid-cols-[16rem_1fr]">
          <ProjectSidebar projects={projects} activeId={projectId} />
          {projectId ? (
            <ProjectDetail projectId={projectId} domains={domains} />
          ) : (
            <p class="text-app-muted">
              {projects.length === 0 ? "No projects on this surface yet." : "Pick a project from the list."}
            </p>
          )}
        </div>
      )}
    </div>
  );
};

export default Projects;
