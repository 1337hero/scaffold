import { cn } from "@/lib/utils.js";
import {
  RiDeleteBinLine,
  RiFlagLine,
  RiPencilLine,
  RiPriceTag3Line,
  RiUserLine,
  RiFolder2Line,
} from "@remixicon/react";

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  year: "numeric",
});

function formatDate(value) {
  if (!value) return "";
  if (/^\d{4}-\d{2}-\d{2}$/.test(String(value))) {
    const [year, month, day] = String(value).split("-").map(Number);
    return dateFormatter.format(new Date(Date.UTC(year, month - 1, day)));
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : dateFormatter.format(date);
}

const NoteItem = ({ note, domain, person, project, expanded, onToggle, onEdit, onDelete, onToggleReview }) => {
  return (
    <article class="rounded-xl border border-app-border bg-card-bg p-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <button type="button" onClick={onToggle} class="min-w-0 flex-1 text-left">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="truncate text-base font-semibold">{note.title}</h2>
            {note.flagForReview && (
              <span class="inline-flex items-center gap-1 rounded-full bg-status-warning/15 px-2 py-0.5 text-[10px] font-bold uppercase text-status-warning">
                <RiFlagLine size={12} /> Review
              </span>
            )}
            {note.reviewAt && (
              <span class="rounded-full border border-app-border px-2 py-0.5 text-[10px] uppercase text-app-muted">
                {formatDate(note.reviewAt)}
              </span>
            )}
          </div>
          <p class="mt-1 line-clamp-2 text-sm text-app-muted">{note.content || "No content."}</p>
        </button>

        <div class="flex shrink-0 gap-1">
          <button
            type="button"
            onClick={onToggleReview}
            aria-label={note.flagForReview ? `Clear review flag for ${note.title}` : `Flag ${note.title} for review`}
            class={cn(
              "rounded-full p-1.5 hover:bg-app-bg",
              note.flagForReview ? "text-status-warning" : "text-app-muted",
            )}
          >
            <RiFlagLine size={16} />
          </button>
          <button type="button" onClick={onEdit} class="rounded-full p-1.5 text-app-muted hover:bg-app-bg" aria-label={`Edit ${note.title}`}>
            <RiPencilLine size={16} />
          </button>
          <button
            type="button"
            onClick={onDelete}
            class="rounded-full p-1.5 text-status-error hover:bg-status-error/10"
            aria-label={`Delete ${note.title}`}
          >
            <RiDeleteBinLine size={16} />
          </button>
        </div>
      </div>

      <div class="mt-3 flex flex-wrap gap-2 text-xs text-app-muted">
        <span>{formatDate(note.updatedAt || note.createdAt)}</span>
        {domain && <span>{domain.Name}</span>}
        {note.source && <span>{note.source}</span>}
        {note.tags && (
          <span class="inline-flex items-center gap-1">
            <RiPriceTag3Line size={13} /> {note.tags}
          </span>
        )}
        {person && (
          <a href={`#/people/${person.id}`} class="inline-flex items-center gap-1 underline decoration-app-border hover:decoration-app-ink">
            <RiUserLine size={13} /> {person.name}
          </a>
        )}
        {project && (
          <a href={`#/projects/${project.id}`} class="inline-flex items-center gap-1 underline decoration-app-border hover:decoration-app-ink">
            <RiFolder2Line size={13} /> {project.name}
          </a>
        )}
      </div>

      {expanded && note.content && (
        <div class="mt-4 whitespace-pre-wrap border-t border-app-border pt-4 text-sm leading-6">
          {note.content}
        </div>
      )}
    </article>
  );
};

export default NoteItem;
