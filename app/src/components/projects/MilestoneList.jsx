import { createMilestone, deleteMilestone, patchMilestone } from "@/api/queries.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine, RiCloseLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const MilestoneList = ({ projectId, milestones, completed, total }) => {
  const queryClient = useQueryClient();
  const [newTitle, setNewTitle] = useState("");

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["project-detail", projectId] });
  const toggleMutation = useMutation({
    mutationFn: ({ id, done }) => patchMilestone(id, { completed: done ? 1 : 0 }),
    onSuccess: invalidate,
  });
  const addMutation = useMutation({
    mutationFn: (title) => createMilestone(projectId, { title, position: milestones.length + 1 }),
    onSuccess: () => {
      setNewTitle("");
      invalidate();
    },
  });
  const removeMutation = useMutation({ mutationFn: deleteMilestone, onSuccess: invalidate });

  const pct = total > 0 ? Math.round((completed / total) * 100) : 0;

  return (
    <section aria-label="Milestones" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <div class="flex items-center justify-between mb-2">
        <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted">Milestones</h2>
        {total > 0 && <span class="text-xs mono text-app-muted">{pct}%</span>}
      </div>
      {total > 0 && (
        <div class="h-1.5 rounded-full bg-app-border mb-3 overflow-hidden">
          <div class="h-full bg-status-success transition-all" style={{ width: `${pct}%` }} />
        </div>
      )}

      <ul class="space-y-1 mb-3">
        {milestones.map((m) => (
          <li key={m.id} class="flex items-center gap-2 group">
            <label class="flex items-center gap-2 flex-1 cursor-pointer">
              <input
                type="checkbox"
                checked={m.completed}
                onChange={() => toggleMutation.mutate({ id: m.id, done: !m.completed })}
                class="accent-[var(--color-status-success)]"
              />
              <span class={cn("text-sm", m.completed && "line-through text-app-muted")}>{m.title}</span>
            </label>
            <button
              type="button"
              onClick={() => removeMutation.mutate(m.id)}
              title="Delete milestone"
              aria-label={`Delete milestone ${m.title}`}
              class="opacity-0 group-hover:opacity-100 text-app-muted hover:text-status-error"
            >
              <RiCloseLine size={15} />
            </button>
          </li>
        ))}
      </ul>

      <form
        class="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (newTitle.trim()) addMutation.mutate(newTitle.trim());
        }}
      >
        <input
          type="text"
          value={newTitle}
          onInput={(e) => setNewTitle(e.currentTarget.value)}
          placeholder="New milestone"
          class="flex-1 px-3 py-1.5 rounded-full bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
        />
        <button
          type="submit"
          class="p-1.5 rounded-full bg-accent text-white hover:bg-accent-hover"
          aria-label="Add milestone"
        >
          <RiAddLine size={16} />
        </button>
      </form>
    </section>
  );
};

export default MilestoneList;
