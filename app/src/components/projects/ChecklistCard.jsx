import { cloneChecklist, createChecklist, checklistTemplatesQuery, patchChecklist } from "@/api/queries.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const Checklist = ({ checklist, onToggleItem }) => {
  const done = checklist.items.filter((i) => i.completed).length;
  return (
    <div class="mb-4 last:mb-0">
      <div class="flex items-center justify-between mb-1">
        <h3 class="text-sm font-semibold">{checklist.title}</h3>
        <span class="text-xs mono text-app-muted">
          {done}/{checklist.items.length}
        </span>
      </div>
      <ul class="space-y-0.5">
        {checklist.items.map((item, idx) => (
          <li key={idx}>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={Boolean(item.completed)}
                onChange={() => onToggleItem(checklist, idx)}
                class="accent-[var(--color-status-success)]"
              />
              <span class={cn("text-sm", item.completed && "line-through text-app-muted")}>{item.text}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
};

const ChecklistCard = ({ projectId, checklists }) => {
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newItems, setNewItems] = useState("");
  const { data: templates = [] } = useQuery({ ...checklistTemplatesQuery, enabled: adding });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["project-detail", projectId] });
  const toggleMutation = useMutation({
    mutationFn: ({ id, items }) => patchChecklist(id, { items: JSON.stringify(items) }),
    onSuccess: invalidate,
  });
  const createMutation = useMutation({
    mutationFn: () =>
      createChecklist(projectId, {
        title: newTitle.trim(),
        items: JSON.stringify(
          newItems
            .split("\n")
            .map((t) => t.trim())
            .filter(Boolean)
            .map((text) => ({ text, completed: false })),
        ),
      }),
    onSuccess: () => {
      setAdding(false);
      setNewTitle("");
      setNewItems("");
      invalidate();
    },
  });
  const cloneMutation = useMutation({
    mutationFn: (templateId) => cloneChecklist(projectId, templateId),
    onSuccess: invalidate,
  });

  const handleToggleItem = (checklist, idx) => {
    const items = checklist.items.map((item, i) =>
      i === idx ? { ...item, completed: !item.completed } : item,
    );
    toggleMutation.mutate({ id: checklist.id, items });
  };

  return (
    <section aria-label="Checklists" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted">Checklists</h2>
        <button
          type="button"
          onClick={() => setAdding(!adding)}
          class="flex items-center gap-1 text-xs text-accent hover:text-accent-hover"
        >
          <RiAddLine size={14} /> New checklist
        </button>
      </div>

      {checklists.length === 0 && !adding && <p class="text-sm text-app-muted">No checklists yet.</p>}

      {checklists.map((c) => (
        <Checklist key={c.id} checklist={c} onToggleItem={handleToggleItem} />
      ))}

      {adding && (
        <form
          class="mt-3 space-y-2 border-t border-app-border pt-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (newTitle.trim()) createMutation.mutate();
          }}
        >
          {templates.length > 0 && (
            <div class="flex flex-wrap gap-2 items-center">
              <span class="text-xs text-app-muted">Clone template:</span>
              {templates.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => cloneMutation.mutate(t.id)}
                  class="px-3 py-1 rounded-full border border-app-border text-xs hover:bg-app-bg"
                >
                  {t.title}
                </button>
              ))}
            </div>
          )}
          <input
            type="text"
            value={newTitle}
            onInput={(e) => setNewTitle(e.currentTarget.value)}
            placeholder="Checklist title"
            class="w-full px-3 py-1.5 rounded-full bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
          />
          <textarea
            value={newItems}
            onInput={(e) => setNewItems(e.currentTarget.value)}
            placeholder="One item per line"
            rows={4}
            class="w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
          />
          <button
            type="submit"
            class="px-4 py-1.5 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold"
          >
            Add checklist
          </button>
        </form>
      )}
    </section>
  );
};

export default ChecklistCard;
