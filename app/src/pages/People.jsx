import { birthdaysQuery, createPerson, domainsQuery, peopleListQuery } from "@/api/queries.js";
import PeopleFilters from "@/components/people/PeopleFilters.jsx";
import PersonCard from "@/components/people/PersonCard.jsx";
import PersonDetail from "@/components/people/PersonDetail.jsx";
import PersonForm from "@/components/people/PersonForm.jsx";
import {
  birthdayHitsForPerson,
  formatBirthdayHit,
  formatDate,
  isPersonSlipping,
  quietDays,
  relationshipLabels,
} from "@/components/people/personUtils.js";
import { useSurface } from "@/hooks/useSurface.jsx";
import { navigate } from "@/hooks/useRoute.js";
import { cn } from "@/lib/utils.js";
import { RiAddLine, RiAlarmWarningLine, RiCake2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

function applyFilters(people, filters) {
  const q = filters.search.trim().toLowerCase();
  return people.filter((person) => {
    if (q && !person.name.toLowerCase().includes(q)) return false;
    if (filters.relationship && person.relationship !== filters.relationship) return false;
    if (filters.domainId && String(person.domainId ?? "") !== filters.domainId) return false;
    return true;
  });
}

function comparePeople(a, b) {
  const aSlip = isPersonSlipping(a) ? 0 : 1;
  const bSlip = isPersonSlipping(b) ? 0 : 1;
  if (aSlip !== bSlip) return aSlip - bSlip;
  return a.name.localeCompare(b.name);
}

const PeopleList = ({ people, birthdayHits, domains, activeId }) => {
  const domainById = Object.fromEntries(domains.map((domain) => [domain.ID, domain]));

  return (
    <div class="overflow-x-auto rounded-xl border border-app-border bg-card-bg/30">
      <table class="w-full text-left">
        <thead>
          <tr class="border-b border-app-border text-xs uppercase tracking-wide text-app-muted">
            <th class="px-3 py-2">Name</th>
            <th class="px-3 py-2">Relationship</th>
            <th class="px-3 py-2">Domain</th>
            <th class="px-3 py-2">Birthday</th>
            <th class="px-3 py-2">Last Touch</th>
            <th class="px-3 py-2">Cadence</th>
          </tr>
        </thead>
        <tbody>
          {people.map((person) => {
            const hits = birthdayHitsForPerson(person, birthdayHits);
            const slipping = isPersonSlipping(person);
            const daysQuiet = quietDays(person);
            return (
              <tr
                key={person.id}
                class={cn(
                  "border-b border-app-border hover:bg-card-bg",
                  activeId === person.id && "bg-card-bg",
                )}
              >
                <td class="px-3 py-2 font-medium">
                  <a href={`#/people/${person.id}`} class="underline decoration-transparent hover:decoration-app-border">
                    {person.name}
                  </a>
                  {slipping && (
                    <span class="ml-2 inline-flex items-center gap-1 rounded-full bg-status-warning/15 px-2 py-0.5 text-[10px] font-bold uppercase text-status-warning">
                      <RiAlarmWarningLine size={11} /> {daysQuiet}d
                    </span>
                  )}
                </td>
                <td class="px-3 py-2 text-sm text-app-muted">{relationshipLabels[person.relationship] ?? ""}</td>
                <td class="px-3 py-2 text-sm text-app-muted">{domainById[person.domainId]?.Name ?? ""}</td>
                <td class="px-3 py-2 text-sm">
                  {hits[0] ? (
                    <span class="inline-flex items-center gap-1 text-status-error">
                      <RiCake2Line size={14} /> {formatBirthdayHit(hits[0])}
                    </span>
                  ) : (
                    formatDate(person.birthday)
                  )}
                </td>
                <td class="px-3 py-2 text-sm text-app-muted">{formatDate(person.lastInteractionAt)}</td>
                <td class="px-3 py-2 text-sm text-app-muted">{person.contactCadenceDays}d</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

const BirthdayStrip = ({ hits }) => {
  if (hits.length === 0) return null;
  return (
    <div class="flex flex-wrap gap-2 rounded-xl border border-status-error/20 bg-status-error/5 p-3">
      {hits.map((hit) => (
        <a
          key={`${hit.personId}-${hit.kind}-${hit.name}-${hit.date}`}
          href={`#/people/${hit.personId}`}
          class={cn(
            "inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold transition-colors",
            hit.urgency === "today"
              ? "bg-status-error text-white"
              : "bg-card-bg text-status-error hover:bg-status-error/10",
          )}
        >
          <RiCake2Line size={14} />
          {formatBirthdayHit(hit)}
          <span class="font-normal opacity-80">({hit.kind})</span>
        </a>
      ))}
    </div>
  );
};

const People = ({ personId }) => {
  const { surface } = useSurface();
  const queryClient = useQueryClient();

  const [creating, setCreating] = useState(false);
  const [view, setView] = useState("grid");
  const [filters, setFilters] = useState({ search: "", relationship: "", domainId: "" });

  const { data: people = [], isLoading, error } = useQuery(peopleListQuery(surface));
  const { data: domains = [] } = useQuery(domainsQuery);
  const { data: birthdayHits = [] } = useQuery(birthdaysQuery(7));

  const createMutation = useMutation({
    mutationFn: createPerson,
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["people-list"] });
      queryClient.invalidateQueries({ queryKey: ["birthdays"] });
      setCreating(false);
      navigate(`/people/${created.ID}`);
    },
  });

  const domainById = Object.fromEntries(domains.map((domain) => [domain.ID, domain]));
  const peopleById = Object.fromEntries(people.map((person) => [person.id, person]));
  const visible = applyFilters(people, filters).sort(comparePeople);
  const visibleById = Object.fromEntries(visible.map((person) => [person.id, person]));
  const visibleBirthdays = birthdayHits.filter((hit) => visibleById[hit.personId]);

  if (error) return <p class="text-status-error">Couldn't load people: {error.message}</p>;

  return (
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="font-serif text-3xl font-semibold italic tracking-tight">People</h1>
          <p class="mt-1 text-sm text-app-muted">
            {people.length} contacts · {people.filter(isPersonSlipping).length} slipping
          </p>
        </div>
        <button
          type="button"
          onClick={() => setCreating(!creating)}
          class="flex items-center gap-1.5 rounded-full bg-accent px-4 py-2 text-sm font-semibold text-white hover:bg-accent-hover"
        >
          <RiAddLine size={16} /> New Person
        </button>
      </div>

      {creating && (
        <PersonForm
          domains={domains}
          surface={surface}
          submitting={createMutation.isPending}
          onSubmit={(data) => createMutation.mutate(data)}
          onCancel={() => setCreating(false)}
        />
      )}

      <BirthdayStrip hits={visibleBirthdays} />

      <PeopleFilters filters={filters} onChange={setFilters} domains={domains} view={view} onViewChange={setView} />

      {isLoading ? (
        <p class="text-app-muted">Loading people…</p>
      ) : (
        <div class={cn("grid gap-6", personId && "xl:grid-cols-[minmax(0,1fr)_28rem]")}>
          <div class="min-w-0 space-y-3">
            {visible.length === 0 ? (
              <p class="text-app-muted">No people match this surface and filter set.</p>
            ) : view === "grid" ? (
              <div class="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
                {visible.map((person) => (
                  <PersonCard
                    key={person.id}
                    person={person}
                    domain={domainById[person.domainId]}
                    birthdayHits={birthdayHitsForPerson(person, visibleBirthdays)}
                    active={person.id === personId}
                  />
                ))}
              </div>
            ) : (
              <PeopleList people={visible} birthdayHits={visibleBirthdays} domains={domains} activeId={personId} />
            )}
          </div>

          {personId && (
            <aside class="min-w-0 xl:sticky xl:top-20 xl:self-start">
              <PersonDetail
                personId={personId}
                domains={domains}
                birthdayHits={birthdayHitsForPerson(peopleById[personId] ?? { id: personId }, birthdayHits)}
              />
            </aside>
          )}
        </div>
      )}
    </div>
  );
};

export default People;
