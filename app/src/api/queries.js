import { apiFetch } from "./fetch.js";

const SCAFFOLD_PROJECT = { name: "Scaffold", color: "amber" };

function parseMicroSteps(nullString) {
  if (!nullString?.Valid || !nullString.String) return [];
  try {
    const steps = JSON.parse(nullString.String);
    return Array.isArray(steps)
      ? steps.map((s, i) => ({ id: s.id ?? i, text: s.text ?? "", done: s.done ?? false }))
      : [];
  } catch {
    return [];
  }
}

function adaptDeskItem(item) {
  return {
    id: item.ID,
    title: item.Title ?? "",
    project: SCAFFOLD_PROJECT,
    done: item.Status === "done",
    microSteps: parseMicroSteps(item.MicroSteps),
  };
}

function adaptDeskItems(items) {
  if (!items?.length) return { theOne: null, tasks: [], doneToday: [] };

  const sorted = [...items].sort((a, b) => a.Position - b.Position);
  const oneRaw = sorted.find((i) => i.Position === 1) ?? null;
  const tasksRaw = sorted.filter((i) => i.Position > 1);

  return {
    theOne: oneRaw ? adaptDeskItem(oneRaw) : null,
    tasks: tasksRaw.map((item, i) => ({ ...adaptDeskItem(item), num: i + 2 })),
    doneToday: [],
  };
}

export const deskQuery = {
  queryKey: ["desk"],
  queryFn: async () => {
    const items = await apiFetch("/api/desk");
    return adaptDeskItems(items);
  },
};

// INBOX

const TRIAGE_GROUPS = [
  { action: "do",        label: "Do",        color: "amber"  },
  { action: "explore",   label: "Explore",   color: "purple" },
  { action: "reference", label: "Reference", color: "cyan"   },
];

const ACTION_TYPE = { do: "task", explore: "idea", reference: "note" };

function formatCaptureTime(createdAt) {
  if (!createdAt) return "";
  const d = new Date(createdAt);
  if (isNaN(d)) return "";

  const now = new Date();
  const isToday =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();

  if (isToday) {
    const h = d.getHours() % 12 || 12;
    const ampm = d.getHours() >= 12 ? "p" : "a";
    const m = d.getMinutes();
    return m === 0 ? `${h}${ampm}` : `${h}:${d.getMinutes().toString().padStart(2, "0")}${ampm}`;
  }

  const day = d.toLocaleDateString("en-US", { weekday: "short" });
  const h = d.getHours() % 12 || 12;
  const ampm = d.getHours() >= 12 ? "p" : "a";
  return `${day} ${h}${ampm}`;
}

function adaptCapture(capture, action) {
  return {
    id: capture.ID,
    triageAction: action,
    confirmed: capture.Confirmed === 1,
    type: capture.Type ?? ACTION_TYPE[action] ?? "note",
    title: capture.Title ?? "",
    summary: capture.Summary ?? "",
    time: formatCaptureTime(capture.CreatedAt),
    cardType: ACTION_TYPE[action] ?? "note",
  };
}

function adaptInboxCaptures(captures) {
  if (!captures?.length) return [];

  const buckets = Object.fromEntries(TRIAGE_GROUPS.map((g) => [g.action, []]));

  for (const c of captures) {
    const action = c.TriageAction?.Valid ? c.TriageAction.String : "do";
    const key = buckets[action] !== undefined ? action : "do";
    buckets[key].push(adaptCapture(c, key));
  }

  return TRIAGE_GROUPS
    .filter((g) => buckets[g.action].length > 0)
    .map((g) => ({ id: g.action, label: g.label, color: g.color, items: buckets[g.action] }));
}

export const inboxQuery = {
  queryKey: ["inbox"],
  queryFn: async () => {
    const captures = await apiFetch("/api/inbox");
    return adaptInboxCaptures(captures);
  },
};

async function postJSON(path, body) {
  return apiFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export async function confirmInboxCapture(captureID) {
  return apiFetch(`/api/inbox/${encodeURIComponent(captureID)}/confirm`, {
    method: "POST",
  });
}

export async function archiveInboxCapture(captureID) {
  return apiFetch(`/api/inbox/${encodeURIComponent(captureID)}/archive`, {
    method: "POST",
  });
}

export async function overrideInboxCapture(captureID, payload) {
  return postJSON(`/api/inbox/${encodeURIComponent(captureID)}/override`, payload);
}

export async function createCapture(text) {
  return postJSON("/api/capture", { text });
}

// DOMAINS / MAP

export const domainsQuery = {
  queryKey: ["domains"],
  queryFn: async () => apiFetch("/api/domains"),
};

export const domainDetailQuery = (id) => ({
  queryKey: ["domain", id],
  queryFn: async () => {
    const dump = id === "dump" || id === 0 || id === "0";
    if (!dump) {
      return apiFetch(`/api/domains/${encodeURIComponent(id)}`);
    }

    const dumpData = await apiFetch("/api/domains/dump");
    const captures = Array.isArray(dumpData?.captures) ? dumpData.captures : [];
    const memories = Array.isArray(dumpData?.memories) ? dumpData.memories : [];
    const count = Number.isFinite(dumpData?.count) ? dumpData.count : captures.length + memories.length;

    return {
      domain: {
        id: 0,
        name: "The Dump",
        importance: 1,
      },
      desk_items: [],
      open_captures: captures,
      recent_memories: memories,
      drift_state: "cold",
      drift_label: `${count} items`,
    };
  },
});

export async function patchDomain(id, body) {
  return apiFetch(`/api/domains/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// CALENDAR

export const calendarQuery = {
  queryKey: ["calendar"],
  queryFn: async () => apiFetch("/api/calendar/upcoming", { allow404: true, fallback: [] }),
  staleTime: 5 * 60 * 1000,
};
