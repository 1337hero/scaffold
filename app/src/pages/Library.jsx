import {
  createNote,
  deleteNote,
  domainsQuery,
  libraryNotesQuery,
  peopleListQuery,
  projectsListQuery,
  updateNote,
} from "@/api/queries.js";
import LibraryFilters from "@/components/library/LibraryFilters.jsx";
import NoteForm from "@/components/library/NoteForm.jsx";
import NoteItem from "@/components/library/NoteItem.jsx";
import { useSurface } from "@/hooks/useSurface.jsx";
import { cn } from "@/lib/utils.js";
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const tabs = [
  { kind: "note", label: "Notes", path: "#/library" },
  { kind: "journal", label: "Journal", path: "#/library/journal" },
  { kind: "quote", label: "Quotes", path: "#/library/quotes" },
];

const kindLabels = { note: "Note", journal: "Journal", quote: "Quote" };

function activeKind(tabParam) {
  if (tabParam === "journal") return "journal";
  if (tabParam === "quotes") return "quote";
  return "note";
}

function withinDateRange(note, filters) {
  if (!filters.from && !filters.to) return true;
  const day = (note.createdAt || note.updatedAt || "").slice(0, 10);
  if (!day) return true;
  if (filters.from && day < filters.from) return false;
  if (filters.to && day > filters.to) return false;
  return true;
}

function groupBySource(notes) {
  return notes.reduce((groups, note) => {
    const key = note.source || "Unsorted";
    groups[key] = groups[key] ? [...groups[key], note] : [note];
    return groups;
  }, {});
}

const Library = ({ tab }) => {
  const { surface } = useSurface();
  const queryClient = useQueryClient();
  const kind = activeKind(tab);

  const [filters, setFilters] = useState({ q: "", tags: "", source: "", flagForReview: "", from: "", to: "" });
  const [formNote, setFormNote] = useState(null);
  const [expandedId, setExpandedId] = useState(null);

  const queryFilters = {
    kind,
    surface,
    q: filters.q,
    tags: kind === "note" ? filters.tags : "",
    source: kind !== "journal" ? filters.source : "",
    flagForReview: kind === "note" ? filters.flagForReview : "",
  };

  const { data: notes = [], isLoading, error } = useQuery(libraryNotesQuery(queryFilters));
  const { data: domains = [] } = useQuery(domainsQuery);
  const { data: people = [] } = useQuery(peopleListQuery(surface));
  const { data: projects = [] } = useQuery(projectsListQuery(surface));

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["library-notes"] });
    queryClient.invalidateQueries({ queryKey: ["person-notes"] });
    queryClient.invalidateQueries({ queryKey: ["today"] });
  };

  const saveMutation = useMutation({
    mutationFn: ({ id, data }) => (id ? updateNote(id, data) : createNote(data)),
    onSuccess: () => {
      invalidate();
      setFormNote(null);
    },
  });
  const deleteMutation = useMutation({ mutationFn: deleteNote, onSuccess: invalidate });
  const reviewMutation = useMutation({
    mutationFn: (note) => updateNote(note.id, { flag_for_review: !note.flagForReview }),
    onSuccess: invalidate,
  });

  const domainById = Object.fromEntries(domains.map((domain) => [domain.ID, domain]));
  const personById = Object.fromEntries(people.map((person) => [person.id, person]));
  const projectById = Object.fromEntries(projects.map((project) => [project.id, project]));
  const visible = notes.filter((note) => (kind === "journal" ? withinDateRange(note, filters) : true));
  const quoteGroups = kind === "quote" ? groupBySource(visible) : {};

  const handleDelete = (note) => {
    if (confirm(`Delete ${note.title}?`)) deleteMutation.mutate(note.id);
  };

  const renderNote = (note) => (
    <NoteItem
      key={note.id}
      note={note}
      domain={domainById[note.domainId]}
      person={personById[note.personId]}
      project={projectById[note.projectId]}
      expanded={expandedId === note.id}
      onToggle={() => setExpandedId(expandedId === note.id ? null : note.id)}
      onEdit={() => setFormNote(note)}
      onDelete={() => handleDelete(note)}
      onToggleReview={() => reviewMutation.mutate(note)}
    />
  );

  if (error) return <p class="text-status-error">Couldn't load library: {error.message}</p>;

  return (
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="font-serif text-3xl font-semibold italic tracking-tight">Library</h1>
          <p class="mt-1 text-sm text-app-muted">
            {visible.length} {kind === "quote" ? "quotes" : kind === "journal" ? "entries" : "notes"}
          </p>
        </div>
        <button
          type="button"
          onClick={() => setFormNote(formNote === "new" ? null : "new")}
          class="flex items-center gap-1.5 rounded-full bg-accent px-4 py-2 text-sm font-semibold text-white hover:bg-accent-hover"
        >
          <RiAddLine size={16} /> New {kindLabels[kind]}
        </button>
      </div>

      <nav class="flex flex-wrap gap-2" aria-label="Library tabs">
        {tabs.map((tabItem) => (
          <a
            key={tabItem.kind}
            href={tabItem.path}
            class={cn(
              "rounded-full border px-4 py-2 text-sm font-semibold transition-colors",
              kind === tabItem.kind
                ? "border-accent bg-accent text-white"
                : "border-app-border bg-card-bg text-app-muted hover:text-app-ink",
            )}
            aria-current={kind === tabItem.kind ? "page" : undefined}
          >
            {tabItem.label}
          </a>
        ))}
      </nav>

      {formNote && (
        <NoteForm
          note={formNote === "new" ? null : formNote}
          kind={kind}
          domains={domains}
          people={people}
          projects={projects}
          submitting={saveMutation.isPending}
          onSubmit={(data) => saveMutation.mutate({ id: formNote === "new" ? null : formNote.id, data })}
          onCancel={() => setFormNote(null)}
        />
      )}

      <LibraryFilters kind={kind} filters={filters} onChange={setFilters} />

      {isLoading ? (
        <p class="text-app-muted">Loading library…</p>
      ) : visible.length === 0 ? (
        <p class="text-app-muted">No {kind === "quote" ? "quotes" : kind === "journal" ? "entries" : "notes"} match.</p>
      ) : kind === "quote" ? (
        <div class="space-y-6">
          {Object.entries(quoteGroups).map(([source, items]) => (
            <section key={source} class="space-y-3">
              <h2 class="text-xs font-bold uppercase tracking-wide text-app-muted">{source}</h2>
              <div class="space-y-3">{items.map(renderNote)}</div>
            </section>
          ))}
        </div>
      ) : (
        <div class="space-y-3">{visible.map(renderNote)}</div>
      )}
    </div>
  );
};

export default Library;
