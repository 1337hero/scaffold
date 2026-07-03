import { checklistTemplatesQuery } from "@/api/queries.js";
import { cn } from "@/lib/utils.js";
import { useQuery } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const inputClass =
  "w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent";

const ProjectForm = ({ project, domains, surface, onSubmit, onCancel, submitting }) => {
  const isNew = !project;
  const [form, setForm] = useState({
    name: project?.name ?? "",
    type: project?.type ?? "project",
    status: project?.status ?? "active",
    domainId: project?.domainId != null ? String(project.domainId) : "",
    startDate: project?.startDate ?? "",
    endDate: project?.endDate ?? "",
    description: project?.description ?? "",
  });
  const [templateIds, setTemplateIds] = useState([]);
  const { data: templates = [] } = useQuery({ ...checklistTemplatesQuery, enabled: isNew });

  const set = (key) => (e) => setForm({ ...form, [key]: e.currentTarget.value });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    const data = {
      name: form.name.trim(),
      type: form.type,
      status: form.status,
      domain_id: form.domainId ? Number(form.domainId) : null,
      start_date: form.startDate || null,
      // Areas are ongoing by definition — never carry an end date.
      end_date: form.type === "project" && form.endDate ? form.endDate : null,
      description: form.description || null,
      ...(isNew && { surface }),
    };
    onSubmit(data, templateIds);
  };

  return (
    <form onSubmit={handleSubmit} class="p-4 rounded-xl bg-card-bg border border-app-border space-y-3">
      <input type="text" value={form.name} onInput={set("name")} placeholder="Name" class={inputClass} required />
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <select value={form.type} onChange={set("type")} class={inputClass} aria-label="Type" disabled={!isNew}>
          <option value="project">Project</option>
          <option value="area">Area</option>
          <option value="retainer">Retainer</option>
        </select>
        <select value={form.status} onChange={set("status")} class={inputClass} aria-label="Status">
          <option value="active">Active</option>
          <option value="on_hold">On hold</option>
          <option value="completed">Completed</option>
          <option value="archived">Archived</option>
        </select>
        <select value={form.domainId} onChange={set("domainId")} class={inputClass} aria-label="Domain">
          <option value="">No domain</option>
          {domains.map((d) => (
            <option key={d.ID} value={String(d.ID)}>
              {d.Name}
            </option>
          ))}
        </select>
        <div class={cn("grid gap-2", form.type === "project" ? "grid-cols-2" : "grid-cols-1")}>
          <input type="date" value={form.startDate} onInput={set("startDate")} class={inputClass} aria-label="Start date" />
          {form.type === "project" && (
            <input type="date" value={form.endDate} onInput={set("endDate")} class={inputClass} aria-label="End date" />
          )}
        </div>
      </div>
      <textarea
        value={form.description}
        onInput={set("description")}
        placeholder="Description (optional)"
        rows={2}
        class={inputClass}
      />

      {isNew && templates.length > 0 && (
        <div>
          <p class="text-xs font-semibold text-app-muted mb-1">Start with checklist templates</p>
          <div class="flex flex-wrap gap-2">
            {templates.map((t) => {
              const selected = templateIds.includes(t.id);
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() =>
                    setTemplateIds(selected ? templateIds.filter((id) => id !== t.id) : [...templateIds, t.id])
                  }
                  class={cn(
                    "px-3 py-1 rounded-full border text-xs transition-colors",
                    selected ? "bg-accent text-white border-accent" : "border-app-border hover:bg-app-bg",
                  )}
                  aria-pressed={selected}
                >
                  {t.title}
                </button>
              );
            })}
          </div>
        </div>
      )}

      <div class="flex gap-2">
        <button
          type="submit"
          disabled={submitting}
          class="px-4 py-2 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold disabled:opacity-50"
        >
          {isNew ? "Create" : "Save"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          class="px-4 py-2 rounded-full border border-app-border text-sm hover:bg-app-bg"
        >
          Cancel
        </button>
      </div>
    </form>
  );
};

export default ProjectForm;
