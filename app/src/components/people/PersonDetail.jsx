import {
  createNote,
  deletePerson,
  logInteraction,
  patchPerson,
  personDetailQuery,
  personInteractionsQuery,
  personNotesQuery,
} from "@/api/queries.js";
import { navigate } from "@/hooks/useRoute.js";
import { cn } from "@/lib/utils.js";
import {
  RiAlarmWarningLine,
  RiCake2Line,
  RiChatNewLine,
  RiDeleteBinLine,
  RiFileListLine,
  RiPencilLine,
  RiStickyNoteAddLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";
import PersonForm from "./PersonForm.jsx";
import {
  formatBirthdayHit,
  formatDate,
  isPersonSlipping,
  quietDays,
  relationshipLabels,
} from "./personUtils.js";

const inputClass =
  "w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent";

function todayISO() {
  const now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  return now.toISOString().slice(0, 10);
}

const InfoRow = ({ label, value }) => {
  if (!value) return null;
  return (
    <div>
      <dt class="text-[10px] font-bold uppercase tracking-wide text-app-muted">{label}</dt>
      <dd class="mt-0.5 text-sm">{value}</dd>
    </div>
  );
};

const PersonDetail = ({ personId, domains, birthdayHits }) => {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [logForm, setLogForm] = useState({ date: todayISO(), summary: "", followUp: "", followUpDate: "" });
  const [noteForm, setNoteForm] = useState({ title: "", content: "" });

  const { data: person, isLoading, error } = useQuery(personDetailQuery(personId));
  const { data: interactions = [] } = useQuery(personInteractionsQuery(personId));
  const { data: notes = [] } = useQuery(personNotesQuery(personId));

  const invalidatePerson = () => {
    queryClient.invalidateQueries({ queryKey: ["people-list"] });
    queryClient.invalidateQueries({ queryKey: ["person-detail", personId] });
    queryClient.invalidateQueries({ queryKey: ["birthdays"] });
    queryClient.invalidateQueries({ queryKey: ["today"] });
  };

  const editMutation = useMutation({
    mutationFn: (updates) => patchPerson(personId, updates),
    onSuccess: () => {
      setEditing(false);
      invalidatePerson();
    },
  });
  const deleteMutation = useMutation({
    mutationFn: () => deletePerson(personId),
    onSuccess: () => {
      invalidatePerson();
      navigate("/people");
    },
  });
  const interactionMutation = useMutation({
    mutationFn: (data) => logInteraction(personId, data),
    onSuccess: () => {
      setLogForm({ date: todayISO(), summary: "", followUp: "", followUpDate: "" });
      queryClient.invalidateQueries({ queryKey: ["person-interactions", personId] });
      invalidatePerson();
    },
  });
  const noteMutation = useMutation({
    mutationFn: (data) => createNote({ ...data, person_id: personId, kind: "note" }),
    onSuccess: () => {
      setNoteForm({ title: "", content: "" });
      queryClient.invalidateQueries({ queryKey: ["person-notes", personId] });
    },
  });

  const domain = person ? domains.find((d) => d.ID === person.domainId) : null;
  const slipping = person ? isPersonSlipping(person) : false;
  const daysQuiet = person ? quietDays(person) : null;

  const handleLogSubmit = (e) => {
    e.preventDefault();
    if (!logForm.summary.trim()) return;
    interactionMutation.mutate({
      date: logForm.date || null,
      summary: logForm.summary.trim(),
      follow_up: logForm.followUp || null,
      follow_up_date: logForm.followUpDate || null,
    });
  };

  const handleNoteSubmit = (e) => {
    e.preventDefault();
    if (!noteForm.title.trim()) return;
    noteMutation.mutate({
      title: noteForm.title.trim(),
      content: noteForm.content || null,
    });
  };

  const handleDelete = () => {
    if (confirm(`Delete ${person.name}?`)) deleteMutation.mutate();
  };

  if (isLoading) return <p class="text-app-muted">Loading person…</p>;
  if (error) return <p class="text-status-error">Couldn't load person: {error.message}</p>;
  if (!person) return <p class="text-app-muted">Person not found.</p>;

  if (editing) {
    return (
      <PersonForm
        person={person}
        domains={domains}
        submitting={editMutation.isPending}
        onSubmit={(updates) => editMutation.mutate(updates)}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <div class="space-y-4">
      <section class="rounded-xl border border-app-border bg-card-bg p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="font-serif text-2xl font-semibold italic tracking-tight">{person.name}</h1>
              <span class="rounded-full bg-app-border px-2 py-0.5 text-[10px] font-bold uppercase">
                {relationshipLabels[person.relationship] ?? "Unsorted"}
              </span>
              {slipping && (
                <a
                  href="#/today"
                  class="inline-flex items-center gap-1 rounded-full bg-status-warning/15 px-2 py-0.5 text-[10px] font-bold uppercase text-status-warning"
                >
                  <RiAlarmWarningLine size={12} /> {daysQuiet}d quiet
                </a>
              )}
            </div>
            <p class="mt-1 text-sm text-app-muted">
              {[person.surface === "business" ? "BusinessOS" : "LifeOS", domain?.Name].filter(Boolean).join(" · ")}
            </p>
          </div>
          <div class="flex shrink-0 gap-2">
            <button
              type="button"
              onClick={() => setEditing(true)}
              class="inline-flex items-center gap-1 rounded-full border border-app-border px-3 py-1.5 text-xs hover:bg-app-bg"
            >
              <RiPencilLine size={13} /> Edit
            </button>
            <button
              type="button"
              onClick={handleDelete}
              class="inline-flex items-center gap-1 rounded-full border border-app-border px-3 py-1.5 text-xs text-status-error hover:bg-status-error/10"
            >
              <RiDeleteBinLine size={13} /> Delete
            </button>
          </div>
        </div>

        {birthdayHits.length > 0 && (
          <div class="mt-4 flex flex-wrap gap-2">
            {birthdayHits.map((hit) => (
              <span
                key={`${hit.kind}-${hit.name}-${hit.date}`}
                class={cn(
                  "inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold",
                  hit.urgency === "today"
                    ? "bg-status-error text-white"
                    : "bg-status-error/10 text-status-error",
                )}
              >
                <RiCake2Line size={14} />
                {formatBirthdayHit(hit)}
                <span class="font-normal opacity-80">({hit.kind})</span>
              </span>
            ))}
          </div>
        )}

        <dl class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <InfoRow label="Birthday" value={formatDate(person.birthday)} />
          <InfoRow label="Anniversary" value={formatDate(person.anniversary)} />
          <InfoRow label="Spouse" value={person.spouse} />
          <InfoRow label="Last Interaction" value={formatDate(person.lastInteractionAt) || "No touch logged"} />
          <InfoRow label="Cadence" value={`${person.contactCadenceDays} days`} />
          <InfoRow label="Linked Notes" value={`${notes.length}`} />
        </dl>

        {person.kids.length > 0 && (
          <div class="mt-5">
            <h2 class="text-xs font-bold uppercase tracking-wide text-app-muted">Kids</h2>
            <ul class="mt-2 grid gap-2 sm:grid-cols-2">
              {person.kids.map((kid, index) => (
                <li key={`${kid.name}-${index}`} class="rounded-lg border border-app-border bg-app-bg/50 px-3 py-2 text-sm">
                  <span class="font-medium">{kid.name || "Unnamed"}</span>
                  {kid.birthday && <span class="text-app-muted"> · {formatDate(kid.birthday)}</span>}
                </li>
              ))}
            </ul>
          </div>
        )}

        {person.notes && (
          <div class="mt-5">
            <h2 class="text-xs font-bold uppercase tracking-wide text-app-muted">Notes</h2>
            <p class="mt-2 whitespace-pre-wrap text-sm leading-6">{person.notes}</p>
          </div>
        )}
      </section>

      <section class="rounded-xl border border-app-border bg-card-bg p-5">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 class="inline-flex items-center gap-2 text-sm font-bold uppercase tracking-wide text-app-muted">
            <RiChatNewLine size={15} /> Interactions
          </h2>
        </div>

        <form onSubmit={handleLogSubmit} class="mb-4 space-y-3 rounded-xl border border-app-border bg-app-bg/40 p-3">
          <div class="grid gap-3 sm:grid-cols-[10rem_1fr]">
            <input
              type="date"
              name="interaction-date"
              value={logForm.date}
              onInput={(e) => setLogForm({ ...logForm, date: e.currentTarget.value })}
              aria-label="Interaction date"
              class={inputClass}
            />
            <input
              type="text"
              name="interaction-summary"
              value={logForm.summary}
              onInput={(e) => setLogForm({ ...logForm, summary: e.currentTarget.value })}
              placeholder="Conversation summary…"
              autocomplete="off"
              aria-label="Interaction summary"
              class={inputClass}
              required
            />
          </div>
          <div class="grid gap-3 sm:grid-cols-[1fr_10rem]">
            <input
              type="text"
              name="follow-up"
              value={logForm.followUp}
              onInput={(e) => setLogForm({ ...logForm, followUp: e.currentTarget.value })}
              placeholder="Follow-up…"
              autocomplete="off"
              aria-label="Follow-up"
              class={inputClass}
            />
            <input
              type="date"
              name="follow-up-date"
              value={logForm.followUpDate}
              onInput={(e) => setLogForm({ ...logForm, followUpDate: e.currentTarget.value })}
              aria-label="Follow-up date"
              class={inputClass}
            />
          </div>
          <button
            type="submit"
            disabled={interactionMutation.isPending}
            class="rounded-full bg-accent px-4 py-2 text-sm font-semibold text-white hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
          >
            Log Interaction
          </button>
        </form>

        {interactions.length === 0 ? (
          <p class="text-sm text-app-muted">No interactions logged.</p>
        ) : (
          <ol class="space-y-3">
            {interactions.map((interaction) => (
              <li key={interaction.id} class="border-l-2 border-app-border pl-3">
                <div class="flex flex-wrap items-baseline justify-between gap-2">
                  <p class="font-medium">{interaction.summary}</p>
                  <time class="text-xs text-app-muted">{formatDate(interaction.date)}</time>
                </div>
                {interaction.followUp && (
                  <p class="mt-1 text-sm text-app-muted">
                    Follow up: {interaction.followUp}
                    {interaction.followUpDate && ` · ${formatDate(interaction.followUpDate)}`}
                  </p>
                )}
              </li>
            ))}
          </ol>
        )}
      </section>

      <section class="rounded-xl border border-app-border bg-card-bg p-5">
        <h2 class="mb-3 inline-flex items-center gap-2 text-sm font-bold uppercase tracking-wide text-app-muted">
          <RiFileListLine size={15} /> Linked Notes
        </h2>

        <form onSubmit={handleNoteSubmit} class="mb-4 space-y-3 rounded-xl border border-app-border bg-app-bg/40 p-3">
          <input
            type="text"
            name="linked-note-title"
            value={noteForm.title}
            onInput={(e) => setNoteForm({ ...noteForm, title: e.currentTarget.value })}
            placeholder="Note title…"
            autocomplete="off"
            aria-label="Note title"
            class={inputClass}
          />
          <textarea
            name="linked-note-content"
            value={noteForm.content}
            onInput={(e) => setNoteForm({ ...noteForm, content: e.currentTarget.value })}
            placeholder="Note content…"
            rows={2}
            aria-label="Note content"
            class={inputClass}
          />
          <button
            type="submit"
            disabled={noteMutation.isPending}
            class="inline-flex items-center gap-1 rounded-full border border-app-border px-4 py-2 text-sm hover:bg-app-bg disabled:cursor-not-allowed disabled:opacity-50"
          >
            <RiStickyNoteAddLine size={15} /> Add Linked Note
          </button>
        </form>

        {notes.length === 0 ? (
          <p class="text-sm text-app-muted">No linked notes.</p>
        ) : (
          <ul class="space-y-2">
            {notes.map((note) => (
              <li key={note.id} class="rounded-lg border border-app-border px-3 py-2 text-sm">
                <a href="#/library" class="font-medium underline decoration-app-border hover:decoration-app-ink">
                  {note.title}
                </a>
                {note.content && <p class="mt-1 line-clamp-2 text-app-muted">{note.content}</p>}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
};

export default PersonDetail;
