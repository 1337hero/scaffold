import { setTop3, top3CandidatesQuery } from "@/api/queries.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine, RiDraggable, RiStarFill } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const Top3Section = ({ tasks, surface }) => {
  const queryClient = useQueryClient();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [dragIndex, setDragIndex] = useState(null);

  const { data: candidates = [] } = useQuery({
    ...top3CandidatesQuery(surface),
    enabled: pickerOpen,
  });

  const mutation = useMutation({
    mutationFn: setTop3,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["today"] });
      queryClient.invalidateQueries({ queryKey: ["top3-candidates"] });
      setPickerOpen(false);
    },
  });

  const ids = tasks.map((t) => t.id);
  const emptySlots = Math.max(0, 3 - tasks.length);

  const handleUnstar = (id) => mutation.mutate(ids.filter((x) => x !== id));
  const handleStar = (id) => mutation.mutate([...ids, id]);

  const handleDrop = (targetIndex) => {
    if (dragIndex === null || dragIndex === targetIndex) return;
    const next = [...ids];
    const [moved] = next.splice(dragIndex, 1);
    next.splice(targetIndex, 0, moved);
    mutation.mutate(next);
    setDragIndex(null);
  };

  return (
    <section aria-label="Top 3">
      <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3">Top 3</h2>
      <div class="grid gap-3 md:grid-cols-3">
        {tasks.map((task, i) => (
          <div
            key={task.id}
            draggable
            onDragStart={() => setDragIndex(i)}
            onDragOver={(e) => e.preventDefault()}
            onDrop={() => handleDrop(i)}
            class={cn(
              "flex items-start gap-3 p-5 rounded-xl bg-card-bg border border-app-border shadow-sm cursor-grab",
              dragIndex === i && "opacity-50",
            )}
          >
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-accent text-white text-sm font-bold shrink-0">
              {i + 1}
            </span>
            <div class="flex-1 min-w-0">
              <p class="font-medium leading-snug">{task.title}</p>
              {task.dueDate && <p class="text-xs text-app-muted mt-1">due {task.dueDate}</p>}
            </div>
            <button
              type="button"
              onClick={() => handleUnstar(task.id)}
              title="Remove from Top 3"
              aria-label={`Remove ${task.title} from Top 3`}
              class="text-accent hover:text-accent-hover transition-colors"
            >
              <RiStarFill size={18} />
            </button>
            <RiDraggable size={18} class="text-app-muted/50 shrink-0" />
          </div>
        ))}

        {Array.from({ length: emptySlots }, (_, i) => (
          <button
            key={`empty-${i}`}
            type="button"
            onClick={() => setPickerOpen(true)}
            class="flex items-center justify-center gap-2 p-5 rounded-xl border-2 border-dashed border-app-border text-app-muted hover:text-app-ink hover:border-app-muted transition-colors min-h-20"
          >
            <RiAddLine size={18} />
            <span class="text-sm">Add to Top 3</span>
          </button>
        ))}
      </div>

      {pickerOpen && (
        <div class="mt-3 p-4 rounded-xl bg-card-bg border border-app-border">
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-sm font-semibold">Pick a task</h3>
            <button
              type="button"
              onClick={() => setPickerOpen(false)}
              class="text-xs text-app-muted hover:text-app-ink"
            >
              Close
            </button>
          </div>
          {candidates.length === 0 ? (
            <p class="text-sm text-app-muted">No pending tasks on this surface.</p>
          ) : (
            <ul class="divide-y divide-app-border max-h-64 overflow-y-auto">
              {candidates.map((task) => (
                <li key={task.id}>
                  <button
                    type="button"
                    onClick={() => handleStar(task.id)}
                    class="w-full text-left py-2 px-1 hover:bg-app-bg rounded transition-colors text-sm"
                  >
                    {task.title}
                    {task.dueDate && <span class="text-app-muted ml-2 text-xs">due {task.dueDate}</span>}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
};

export default Top3Section;
