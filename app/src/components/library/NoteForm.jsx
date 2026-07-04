import { useState } from "preact/hooks";

const inputClass =
  "w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent";

const labelClass = "space-y-1 text-xs font-semibold uppercase tracking-wide text-app-muted";

const kindLabels = { note: "Note", journal: "Journal", quote: "Quote" };

const NoteForm = ({ note, kind, domains, people, projects, submitting, onSubmit, onCancel }) => {
  const [form, setForm] = useState({
    title: note?.title ?? "",
    content: note?.content ?? "",
    kind: note?.kind ?? kind,
    tags: note?.tags ?? "",
    source: note?.source ?? "",
    domainId: note?.domainId != null ? String(note.domainId) : "",
    personId: note?.personId ?? "",
    projectId: note?.projectId ?? "",
    flagForReview: Boolean(note?.flagForReview),
    reviewAt: note?.reviewAt ?? "",
  });

  const set = (key) => (e) => setForm({ ...form, [key]: e.currentTarget.value });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!form.title.trim()) return;
    onSubmit({
      title: form.title.trim(),
      content: form.content || null,
      kind: form.kind,
      tags: form.tags || null,
      source: form.source || null,
      domain_id: form.domainId ? Number(form.domainId) : null,
      person_id: form.personId || null,
      project_id: form.projectId || null,
      flag_for_review: form.flagForReview,
      review_at: form.reviewAt || null,
    });
  };

  return (
    <form onSubmit={handleSubmit} class="space-y-4 rounded-xl border border-app-border bg-card-bg p-4">
      <div class="grid gap-3 md:grid-cols-[1fr_10rem]">
        <label class={labelClass}>
          Title
          <input
            type="text"
            name="note-title"
            value={form.title}
            onInput={set("title")}
            placeholder={`${kindLabels[form.kind]} title…`}
            autocomplete="off"
            class={inputClass}
            required
          />
        </label>
        <label class={labelClass}>
          Type
          <select name="kind" value={form.kind} onChange={set("kind")} class={inputClass}>
            <option value="note">Note</option>
            <option value="journal">Journal</option>
            <option value="quote">Quote</option>
          </select>
        </label>
      </div>

      <label class={labelClass}>
        {form.kind === "quote" ? "Quote / Reflection" : "Content"}
        <textarea
          name="note-content"
          value={form.content}
          onInput={set("content")}
          placeholder={form.kind === "quote" ? "Passage and reflection…" : "Write the note…"}
          rows={5}
          class={inputClass}
        />
      </label>

      <div class="grid gap-3 md:grid-cols-3">
        <label class={labelClass}>
          Tags
          <input
            type="text"
            name="tags"
            value={form.tags}
            onInput={set("tags")}
            placeholder="comma, separated…"
            autocomplete="off"
            class={inputClass}
          />
        </label>
        <label class={labelClass}>
          Source
          <input
            type="text"
            name="note-source"
            value={form.source}
            onInput={set("source")}
            placeholder="book, podcast, Signal…"
            autocomplete="off"
            class={inputClass}
          />
        </label>
        <label class={labelClass}>
          Review Date
          <input type="date" name="review-at" value={form.reviewAt} onInput={set("reviewAt")} class={inputClass} />
        </label>
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <label class={labelClass}>
          Domain
          <select name="domain" value={form.domainId} onChange={set("domainId")} class={inputClass}>
            <option value="">No domain</option>
            {domains.map((domain) => (
              <option key={domain.ID} value={String(domain.ID)}>
                {domain.Name}
              </option>
            ))}
          </select>
        </label>
        <label class={labelClass}>
          Person
          <select name="person" value={form.personId} onChange={set("personId")} class={inputClass}>
            <option value="">No person</option>
            {people.map((person) => (
              <option key={person.id} value={person.id}>
                {person.name}
              </option>
            ))}
          </select>
        </label>
        <label class={labelClass}>
          Project
          <select name="project" value={form.projectId} onChange={set("projectId")} class={inputClass}>
            <option value="">No project</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <label class="inline-flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={form.flagForReview}
          onChange={(e) => setForm({ ...form, flagForReview: e.currentTarget.checked })}
          class="size-4 accent-accent"
        />
        Flag for review
      </label>

      <div class="flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={submitting}
          class="rounded-full bg-accent px-4 py-2 text-sm font-semibold text-white hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
        >
          {note ? "Save Entry" : `Create ${kindLabels[form.kind]}`}
        </button>
        <button
          type="button"
          onClick={onCancel}
          class="rounded-full border border-app-border px-4 py-2 text-sm hover:bg-app-bg"
        >
          Cancel
        </button>
      </div>
    </form>
  );
};

export default NoteForm;
