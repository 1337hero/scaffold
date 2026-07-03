import { logActivity, projectActivityQuery } from "@/api/queries.js";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "preact/hooks";

const ActivityLog = ({ projectId, isRetainer }) => {
  const queryClient = useQueryClient();
  const [description, setDescription] = useState("");
  const [hours, setHours] = useState("");

  const { data: activity = [] } = useQuery(projectActivityQuery(projectId));

  const logMutation = useMutation({
    mutationFn: () =>
      logActivity(projectId, {
        description: description.trim(),
        ...(hours !== "" && { hours: Number(hours) }),
      }),
    onSuccess: () => {
      setDescription("");
      setHours("");
      queryClient.invalidateQueries({ queryKey: ["project-activity", projectId] });
      queryClient.invalidateQueries({ queryKey: ["project-detail", projectId] });
    },
  });

  return (
    <section aria-label="Activity" class="p-5 rounded-xl bg-card-bg border border-app-border">
      <h2 class="text-sm font-bold uppercase tracking-wide text-app-muted mb-3">Activity</h2>

      <form
        class="flex flex-wrap gap-2 mb-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (description.trim()) logMutation.mutate();
        }}
      >
        <input
          type="text"
          value={description}
          onInput={(e) => setDescription(e.currentTarget.value)}
          placeholder="What got done?"
          class="flex-1 min-w-48 px-3 py-1.5 rounded-full bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
        />
        <input
          type="number"
          step="0.25"
          min="0"
          value={hours}
          onInput={(e) => setHours(e.currentTarget.value)}
          placeholder={isRetainer ? "Hours (billable)" : "Hours"}
          class="w-32 px-3 py-1.5 rounded-full bg-input-bg border border-app-border text-sm focus:outline-none focus:border-accent"
          aria-label="Hours"
        />
        <button
          type="submit"
          disabled={logMutation.isPending}
          class="px-4 py-1.5 rounded-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold disabled:opacity-50"
        >
          Log
        </button>
      </form>

      {activity.length === 0 ? (
        <p class="text-sm text-app-muted">No activity logged yet.</p>
      ) : (
        <ul class="space-y-2">
          {activity.map((a) => (
            <li key={a.id} class="flex items-baseline justify-between gap-3">
              <span class="text-sm">{a.description}</span>
              <span class="text-xs mono text-app-muted shrink-0">
                {a.hours != null && `${a.hours}h · `}
                {a.createdAt.slice(0, 10)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
};

export default ActivityLog;
