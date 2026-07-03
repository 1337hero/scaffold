const selectClass =
  "px-3 py-1.5 rounded-full bg-card-bg border border-app-border text-sm text-app-ink";

const TaskFilters = ({ filters, onChange, projects, domains }) => {
  const set = (key) => (e) => onChange({ ...filters, [key]: e.currentTarget.value });

  const containers = [
    { label: "Projects", type: "project" },
    { label: "Areas", type: "area" },
    { label: "Retainers", type: "retainer" },
  ];

  return (
    <div class="flex flex-wrap gap-2">
      <select value={filters.status} onChange={set("status")} class={selectClass} aria-label="Status">
        <option value="pending">Open</option>
        <option value="done">Done</option>
        <option value="all">All</option>
      </select>

      <select value={filters.projectId} onChange={set("projectId")} class={selectClass} aria-label="Project">
        <option value="">All projects</option>
        {containers.map((c) => {
          const items = projects.filter((p) => p.type === c.type);
          if (items.length === 0) return null;
          return (
            <optgroup key={c.type} label={c.label}>
              {items.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </optgroup>
          );
        })}
      </select>

      <select value={filters.domainId} onChange={set("domainId")} class={selectClass} aria-label="Domain">
        <option value="">All domains</option>
        {domains.map((d) => (
          <option key={d.ID} value={String(d.ID)}>
            {d.Name}
          </option>
        ))}
      </select>

      <select value={filters.due} onChange={set("due")} class={selectClass} aria-label="Due">
        <option value="">Any due date</option>
        <option value="overdue">Overdue</option>
        <option value="today">Due today</option>
        <option value="week">Due this week</option>
      </select>

      <select value={filters.priority} onChange={set("priority")} class={selectClass} aria-label="Priority">
        <option value="">Any priority</option>
        <option value="urgent">Urgent</option>
        <option value="high">High</option>
        <option value="normal">Normal</option>
        <option value="low">Low</option>
      </select>
    </div>
  );
};

export default TaskFilters;
