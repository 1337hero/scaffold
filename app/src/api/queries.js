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

const ACTION_UI = {
  do: {
    type: "task",
    actions: ["Add to desk", "Snooze", "Archive"],
  },
  explore: {
    type: "idea",
    actions: ["Open notebook", "Extract tasks", "Archive"],
  },
  reference: {
    type: "note",
    actions: ["Open notebook", "Archive"],
  },
};

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

function splitCaptureText(rawText) {
  const raw = (rawText ?? "").trim().replace(/\s+/g, " ");
  if (!raw) {
    return { title: "Untitled capture", summary: "Captured from inbox." };
  }
  if (raw.length <= 84) {
    return { title: raw, summary: "Captured from inbox. Ready to triage." };
  }

  const target = 84;
  const window = raw.slice(0, target + 24);
  let cut = Math.max(
    window.lastIndexOf(". "),
    window.lastIndexOf(" - "),
    window.lastIndexOf(" — "),
    window.lastIndexOf("; "),
    window.lastIndexOf(", "),
  );
  if (cut < 40) cut = window.lastIndexOf(" ");
  if (cut < 40) cut = target;

  const title = raw.slice(0, cut).trim();
  const remainder = raw.slice(cut).trim().replace(/^[,.;:-]\s*/, "");
  const summary = remainder || "Captured from inbox. Ready to triage.";
  return { title, summary };
}

function inferCaptureType(raw, source, action) {
  if (/(https?:\/\/|www\.)/i.test(raw)) return "link";
  if (/\b(video|youtube|watch|vimeo)\b/i.test(raw)) return "video";
  if (/\bidea|what if|maybe\b/i.test(raw)) return "idea";
  if (/\barticle|research|paper|spec\b/i.test(raw)) return "article";
  if (source?.startsWith("signal")) return "note";
  return ACTION_UI[action]?.type ?? "note";
}

function adaptCapture(capture, action) {
  const raw = capture.Raw ?? "";
  const { title, summary } = splitCaptureText(raw);
  const ui = ACTION_UI[action] ?? ACTION_UI.do;
  return {
    id: capture.ID,
    type: inferCaptureType(raw, capture.Source ?? "", action),
    title,
    summary,
    time: formatCaptureTime(capture.CreatedAt),
    actions: ui.actions,
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

// NOTEBOOKS — backend not yet implemented

export const notebooksQuery = {
  queryKey: ["notebooks"],
  queryFn: async () => [],
};

export const notebookQuery = (id) => ({
  queryKey: ["notebook", id],
  queryFn: async () => null,
});
