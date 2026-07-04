import { cn } from "@/lib/utils.js";
import { RiAlarmWarningLine, RiCake2Line, RiParentLine } from "@remixicon/react";
import {
  formatBirthdayHit,
  formatDate,
  initials,
  isPersonSlipping,
  quietDays,
  relationshipAccent,
  relationshipLabels,
} from "./personUtils.js";

const PersonCard = ({ person, birthdayHits, domain, active }) => {
  const slipping = isPersonSlipping(person);
  const daysQuiet = quietDays(person);
  const firstBirthday = birthdayHits[0];

  return (
    <a
      href={`#/people/${person.id}`}
      aria-current={active ? "page" : undefined}
      class={cn(
        "block rounded-xl border bg-card-bg p-4 border-l-4 transition-colors hover:border-accent hover:bg-card-bg/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        active ? "border-accent" : "border-app-border",
        relationshipAccent[person.relationship] ?? relationshipAccent.other,
      )}
    >
      <div class="flex items-start gap-3">
        <div class="flex size-11 shrink-0 items-center justify-center rounded-full bg-app-bg font-serif text-lg italic">
          {initials(person.name)}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <h2 class="truncate text-base font-semibold">{person.name}</h2>
              <p class="text-xs text-app-muted">
                {relationshipLabels[person.relationship] ?? "Unsorted"}
                {domain ? ` · ${domain.Name}` : ""}
              </p>
            </div>
            {slipping && (
              <span
                class="inline-flex shrink-0 items-center gap-1 rounded-full bg-status-warning/15 px-2 py-0.5 text-[10px] font-bold uppercase text-status-warning"
                title="Past contact cadence"
              >
                <RiAlarmWarningLine size={12} /> {daysQuiet}d
              </span>
            )}
          </div>

          <div class="mt-4 space-y-2 text-sm">
            {firstBirthday ? (
              <p class="flex items-center gap-2 text-status-error">
                <RiCake2Line size={15} class="shrink-0" />
                <span class="truncate">{formatBirthdayHit(firstBirthday)}</span>
              </p>
            ) : person.birthday ? (
              <p class="flex items-center gap-2 text-app-muted">
                <RiCake2Line size={15} class="shrink-0" />
                <span>{formatDate(person.birthday)}</span>
              </p>
            ) : null}

            {(person.spouse || person.kids.length > 0) && (
              <p class="flex items-center gap-2 text-app-muted">
                <RiParentLine size={15} class="shrink-0" />
                <span class="truncate">
                  {[person.spouse && `Spouse: ${person.spouse}`, person.kids.length > 0 && `${person.kids.length} kids`]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </p>
            )}

            <p class="text-xs text-app-muted">
              {person.lastInteractionAt
                ? `Last touch ${formatDate(person.lastInteractionAt)} · ${person.contactCadenceDays}d cadence`
                : `${person.contactCadenceDays}d cadence · no touch logged`}
            </p>
          </div>
        </div>
      </div>
    </a>
  );
};

export default PersonCard;
