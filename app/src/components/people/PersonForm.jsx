import { RiAddLine, RiCloseLine } from "@remixicon/react";
import { useState } from "preact/hooks";
import { relationshipOptions } from "./personUtils.js";

const inputClass =
  "w-full px-3 py-2 rounded-xl bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent";

const labelClass = "space-y-1 text-xs font-semibold uppercase tracking-wide text-app-muted";

function cleanKids(kids) {
  return kids
    .map((kid) => ({ name: String(kid.name ?? "").trim(), birthday: kid.birthday || "" }))
    .filter((kid) => kid.name || kid.birthday);
}

const PersonForm = ({ person, domains, surface, submitting, onSubmit, onCancel }) => {
  const isNew = !person;
  const [form, setForm] = useState({
    name: person?.name ?? "",
    surface: person?.surface ?? surface,
    domainId: person?.domainId != null ? String(person.domainId) : "",
    relationship: person?.relationship ?? "",
    birthday: person?.birthday ?? "",
    anniversary: person?.anniversary ?? "",
    spouse: person?.spouse ?? "",
    notes: person?.notes ?? "",
    contactCadenceDays: String(person?.contactCadenceDays ?? 90),
  });
  const [kids, setKids] = useState(person?.kids?.length ? person.kids : []);

  const set = (key) => (e) => setForm({ ...form, [key]: e.currentTarget.value });

  const handleKidChange = (index, key, value) => {
    setKids(kids.map((kid, i) => (i === index ? { ...kid, [key]: value } : kid)));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!form.name.trim()) return;

    onSubmit({
      name: form.name.trim(),
      surface: form.surface,
      domain_id: form.domainId ? Number(form.domainId) : null,
      relationship: form.relationship || null,
      birthday: form.birthday || null,
      anniversary: form.anniversary || null,
      spouse: form.spouse || null,
      kids: cleanKids(kids),
      notes: form.notes || null,
      contact_cadence_days: form.contactCadenceDays ? Number(form.contactCadenceDays) : null,
    });
  };

  return (
    <form onSubmit={handleSubmit} class="space-y-4 rounded-xl border border-app-border bg-card-bg p-4">
      <div class="grid gap-3 md:grid-cols-2">
        <label class={labelClass}>
          Name
          <input
            type="text"
            name="person-name"
            value={form.name}
            onInput={set("name")}
            placeholder="Ada Lovelace…"
            autocomplete="off"
            class={inputClass}
            required
          />
        </label>
        <label class={labelClass}>
          Relationship
          <select name="relationship" value={form.relationship} onChange={set("relationship")} class={inputClass}>
            <option value="">Unsorted</option>
            {relationshipOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <label class={labelClass}>
          Surface
          <select name="surface" value={form.surface} onChange={set("surface")} class={inputClass}>
            <option value="life">LifeOS</option>
            <option value="business">BusinessOS</option>
          </select>
        </label>
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
          Cadence Days
          <input
            type="number"
            name="contact-cadence-days"
            inputmode="numeric"
            min="1"
            value={form.contactCadenceDays}
            onInput={set("contactCadenceDays")}
            class={inputClass}
          />
        </label>
      </div>

      <div class="grid gap-3 md:grid-cols-3">
        <label class={labelClass}>
          Birthday
          <input type="date" name="birthday" value={form.birthday} onInput={set("birthday")} class={inputClass} />
        </label>
        <label class={labelClass}>
          Anniversary
          <input
            type="date"
            name="anniversary"
            value={form.anniversary}
            onInput={set("anniversary")}
            class={inputClass}
          />
        </label>
        <label class={labelClass}>
          Spouse
          <input
            type="text"
            name="spouse"
            value={form.spouse}
            onInput={set("spouse")}
            placeholder="Partner name…"
            autocomplete="off"
            class={inputClass}
          />
        </label>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs font-semibold uppercase tracking-wide text-app-muted">Kids</p>
          <button
            type="button"
            onClick={() => setKids([...kids, { name: "", birthday: "" }])}
            class="inline-flex items-center gap-1 rounded-full border border-app-border px-3 py-1 text-xs hover:bg-app-bg"
          >
            <RiAddLine size={13} /> Add Kid
          </button>
        </div>
        {kids.length > 0 && (
          <div class="space-y-2">
            {kids.map((kid, index) => (
              <div key={index} class="grid gap-2 sm:grid-cols-[1fr_10rem_2.5rem]">
                <input
                  type="text"
                  name={`kid-name-${index}`}
                  value={kid.name}
                  onInput={(e) => handleKidChange(index, "name", e.currentTarget.value)}
                  placeholder="Kid name…"
                  autocomplete="off"
                  aria-label="Kid name"
                  class={inputClass}
                />
                <input
                  type="date"
                  name={`kid-birthday-${index}`}
                  value={kid.birthday}
                  onInput={(e) => handleKidChange(index, "birthday", e.currentTarget.value)}
                  aria-label="Kid birthday"
                  class={inputClass}
                />
                <button
                  type="button"
                  onClick={() => setKids(kids.filter((_, i) => i !== index))}
                  aria-label="Remove kid"
                  class="flex size-10 items-center justify-center rounded-full border border-app-border text-app-muted hover:bg-status-error/10 hover:text-status-error"
                >
                  <RiCloseLine size={16} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <label class={labelClass}>
        Notes
        <textarea
          name="notes"
          value={form.notes}
          onInput={set("notes")}
          placeholder="Context, preferences, allergies…"
          rows={3}
          class={inputClass}
        />
      </label>

      <div class="flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={submitting}
          class="rounded-full bg-accent px-4 py-2 text-sm font-semibold text-white hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isNew ? "Create Person" : "Save Person"}
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

export default PersonForm;
