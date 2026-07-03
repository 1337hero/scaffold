import { useState } from "preact/hooks";

const inputClass =
  "w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent";

const TaskForm = ({ task, projects, domains, surface, onSubmit, onCancel, submitting }) => {
  const [form, setForm] = useState({
    title: task?.title ?? "",
    projectId: task?.projectId ?? "",
    domainId: task?.domainId ?? "",
    dueDate: task?.dueDate ?? "",
    priority: task?.priority ?? "normal",
    reminderAt: task?.reminderAt ? task.reminderAt.slice(0, 16) : "",
    context: task?.context ?? "",
  });

  const set = (key) => (e) => setForm({ ...form, [key]: e.currentTarget.value });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!form.title.trim()) return;
    onSubmit({
      title: form.title.trim(),
      project_id: form.projectId || null,
      domain_id: form.domainId ? Number(form.domainId) : null,
      due_date: form.dueDate || null,
      priority: form.priority,
      reminder_at: form.reminderAt ? new Date(form.reminderAt).toISOString() : null,
      context: form.context || null,
      surface,
    });
  };

  return (
    <form onSubmit={handleSubmit} class="p-4 rounded-xl bg-card-bg border border-app-border space-y-3">
      <input
        type="text"
        value={form.title}
        onInput={set("title")}
        placeholder="Task title"
        class={inputClass}
        required
      />
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <select value={form.projectId} onChange={set("projectId")} class={inputClass} aria-label="Project">
          <option value="">No project</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        <select value={form.domainId} onChange={set("domainId")} class={inputClass} aria-label="Domain">
          <option value="">No domain</option>
          {domains.map((d) => (
            <option key={d.ID} value={String(d.ID)}>
              {d.Name}
            </option>
          ))}
        </select>
        <input type="date" value={form.dueDate} onInput={set("dueDate")} class={inputClass} aria-label="Due date" />
        <select value={form.priority} onChange={set("priority")} class={inputClass} aria-label="Priority">
          <option value="low">Low</option>
          <option value="normal">Normal</option>
          <option value="high">High</option>
          <option value="urgent">Urgent</option>
        </select>
      </div>
      <div class="grid gap-3 sm:grid-cols-2">
        <input
          type="datetime-local"
          value={form.reminderAt}
          onInput={set("reminderAt")}
          class={inputClass}
          aria-label="Reminder"
        />
        <input
          type="text"
          value={form.context}
          onInput={set("context")}
          placeholder="Context (optional)"
          class={inputClass}
        />
      </div>
      <div class="flex gap-2">
        <button
          type="submit"
          disabled={submitting}
          class="px-4 py-2 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold disabled:opacity-50"
        >
          {task ? "Save" : "Add task"}
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

export default TaskForm;
