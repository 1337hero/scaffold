import { daysSince } from "@/lib/utils.js";

export const relationshipOptions = [
  { value: "family", label: "Family" },
  { value: "friend", label: "Friend" },
  { value: "colleague", label: "Colleague" },
  { value: "client", label: "Client" },
  { value: "mentor", label: "Mentor" },
  { value: "other", label: "Other" },
];

export const relationshipLabels = Object.fromEntries(
  relationshipOptions.map((option) => [option.value, option.label]),
);

export const relationshipAccent = {
  family: "border-l-status-error",
  friend: "border-l-status-success",
  colleague: "border-l-status-info",
  client: "border-l-accent",
  mentor: "border-l-domain-projects",
  other: "border-l-app-muted",
};

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
});

export function formatDate(value) {
  if (!value) return "";
  const parts = String(value).slice(0, 10).split("-").map(Number);
  if (parts.length !== 3 || parts.some((part) => !Number.isFinite(part))) return value;
  return dateFormatter.format(new Date(Date.UTC(parts[0], parts[1] - 1, parts[2])));
}

export function formatBirthdayHit(hit) {
  if (!hit) return "";
  if (hit.daysUntil === 0) return `${hit.name} today`;
  if (hit.daysUntil === 1) return `${hit.name} tomorrow`;
  return `${hit.name} in ${hit.daysUntil}d`;
}

export function birthdayHitsForPerson(person, birthdayHits) {
  return birthdayHits.filter((hit) => hit.personId === person.id);
}

export function isPersonSlipping(person) {
  return Boolean(person.lastInteractionAt && daysSince(person.lastInteractionAt) > person.contactCadenceDays);
}

export function quietDays(person) {
  return person.lastInteractionAt ? daysSince(person.lastInteractionAt) : null;
}

export function initials(name) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
}
