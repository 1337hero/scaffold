import { taskNotesQuery } from "@/api/queries.js";
import { cn } from "@/lib/utils.js";
import {
  RiCheckLine,
  RiDeleteBinLine,
  RiPencilLine,
  RiRepeatLine,
  RiStarFill,
  RiStarLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

const priorityColors = {
  urgent: "text-status-error",
  high: "text-status-warning",
  normal: "text-app-muted",
  low: "text-app-muted/60",
};

function parseMicroSteps(raw) {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

const TaskRow = ({ task, project, starred, top3Full, expanded, onToggleExpand, onStar, onComplete, onEdit, onDelete }) => {
  const { data: notes = [] } = useQuery({ ...taskNotesQuery(task.id), enabled: expanded });
  const microSteps = expanded ? parseMicroSteps(task.microSteps) : [];
  const done = task.status === "done";

  return (
    <>
      <tr
        class={cn("border-b border-app-border hover:bg-card-bg/60 cursor-pointer", done && "opacity-50")}
        onClick={onToggleExpand}
      >
        <td class="py-2 px-2 w-8" onClick={(e) => e.stopPropagation()}>
          <button
            type="button"
            onClick={onStar}
            disabled={!starred && top3Full}
            title={starred ? "Remove from Top 3" : top3Full ? "Top 3 is full" : "Add to Top 3"}
            aria-label={`${starred ? "Unstar" : "Star"} ${task.title}`}
            class={cn(
              starred ? "text-accent" : "text-app-muted/50 hover:text-accent",
              !starred && top3Full && "opacity-30 cursor-not-allowed",
            )}
          >
            {starred ? <RiStarFill size={17} /> : <RiStarLine size={17} />}
          </button>
        </td>
        <td class={cn("py-2 px-2 font-medium", done && "line-through")}>
          {task.title}
          {task.recurring && (
            <span class="inline-flex items-center gap-1 ml-2 text-xs text-app-muted">
              <RiRepeatLine size={13} /> {task.recurring}
            </span>
          )}
        </td>
        <td class="py-2 px-2 text-sm text-app-muted">{project?.name ?? task.domainName}</td>
        <td class="py-2 px-2 text-sm">{task.dueDate ?? ""}</td>
        <td class={cn("py-2 px-2 text-sm capitalize", priorityColors[task.priority])}>{task.priority}</td>
        <td class="py-2 px-2 w-20" onClick={(e) => e.stopPropagation()}>
          {!done && (
            <button
              type="button"
              onClick={onComplete}
              title="Mark done"
              aria-label={`Complete ${task.title}`}
              class="p-1 rounded-full text-status-success hover:bg-status-success/10"
            >
              <RiCheckLine size={18} />
            </button>
          )}
        </td>
      </tr>

      {expanded && (
        <tr class="border-b border-app-border bg-card-bg/40">
          <td />
          <td colSpan={5} class="py-3 px-2">
            <div class="space-y-2 text-sm">
              {task.context && <p>{task.context}</p>}
              {microSteps.length > 0 && (
                <ul class="list-disc ml-5 space-y-0.5">
                  {microSteps.map((step, i) => (
                    <li key={i} class={cn(step.completed && "line-through text-app-muted")}>
                      {step.text ?? String(step)}
                    </li>
                  ))}
                </ul>
              )}
              {task.reminderAt && (
                <p class="text-app-muted">Reminder: {new Date(task.reminderAt).toLocaleString()}</p>
              )}
              {task.recurring && task.dueDate && (
                <p class="text-app-muted">
                  Repeats {task.recurring} — next {task.dueDate}
                </p>
              )}
              {notes.length > 0 && (
                <div>
                  <p class="text-xs font-semibold text-app-muted mb-1">Linked notes</p>
                  <ul class="space-y-0.5">
                    {notes.map((n) => (
                      <li key={n.id}>
                        <a href="#/library" class="underline decoration-app-border hover:decoration-app-ink">
                          {n.title}
                        </a>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <div class="flex gap-2 pt-1">
                <button
                  type="button"
                  onClick={onEdit}
                  class="flex items-center gap-1 px-3 py-1 rounded-full border border-app-border text-xs hover:bg-app-bg"
                >
                  <RiPencilLine size={13} /> Edit
                </button>
                <button
                  type="button"
                  onClick={onDelete}
                  class="flex items-center gap-1 px-3 py-1 rounded-full border border-app-border text-xs text-status-error hover:bg-status-error/10"
                >
                  <RiDeleteBinLine size={13} /> Delete
                </button>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
};

export default TaskRow;
