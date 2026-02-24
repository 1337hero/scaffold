import { apiFetch } from "./fetch.js"

function ensureArray(value) {
  return Array.isArray(value) ? value : []
}

function asNullString(value) {
  if (value && typeof value === "object" && "Valid" in value) return value
  if (typeof value === "string" && value.trim() !== "") {
    return { String: value, Valid: true }
  }
  return { String: "", Valid: false }
}

function asNullInt64(value) {
  if (value && typeof value === "object" && "Valid" in value) return value
  const n = Number(value)
  if (Number.isFinite(n) && value !== "" && value != null) {
    return { Int64: n, Valid: true }
  }
  return { Int64: 0, Valid: false }
}

function normalizeDomain(domain) {
  if (!domain || typeof domain !== "object") return null
  if ("ID" in domain || "Name" in domain) return domain

  const id = Number(domain.id)
  if (!Number.isFinite(id) || id <= 0) return null

  return {
    ID: id,
    Name: domain.name || "",
    Importance: Number(domain.importance ?? 0),
    LastTouchedAt: domain.last_touched_at || "",
    StatusLine: asNullString(domain.status_line),
    Briefing: asNullString(domain.briefing),
    CreatedAt: domain.created_at || "",
    Icon: asNullString(domain.icon),
    Color: asNullString(domain.color),
    Position: Number(domain.position ?? 0),
    Status: typeof domain.status === "string" && domain.status ? domain.status : "active",
  }
}

function normalizeSearchResult(result) {
  if (!result || typeof result !== "object") return null
  if ("ID" in result || "Type" in result) return result

  return {
    ID: result.id || "",
    Type: result.type || "",
    Title: result.title || "",
    Snippet: result.snippet || "",
    DomainID: asNullInt64(result.domain_id),
    Status: result.status || "",
  }
}

function normalizeCalendarEvent(event) {
  if (!event || typeof event !== "object") return null

  return {
    id: event.id || event.ID || "",
    summary: event.summary || event.Summary || event.title || event.Title || "",
    time: event.time || event.Time || "",
    all_day: Boolean(event.all_day ?? event.AllDay),
    start: event.start || event.Start || "",
    end: event.end || event.End || "",
    htmlLink: event.htmlLink || event.HtmlLink || "",
  }
}

function postJSON(path, body) {
  return apiFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

function putJSON(path, body) {
  return apiFetch(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

// Dashboard

export const dashboardQuery = {
  queryKey: ["dashboard"],
  queryFn: async () => {
    const data = await apiFetch("/api/dashboard")
    if (!data || typeof data !== "object") {
      return {
        TodaysTasks: [],
        OverdueTasks: [],
        GoalsWithProgress: [],
        DomainHealth: [],
        DoneToday: [],
      }
    }
    return {
      ...data,
      TodaysTasks: ensureArray(data.TodaysTasks),
      OverdueTasks: ensureArray(data.OverdueTasks),
      GoalsWithProgress: ensureArray(data.GoalsWithProgress),
      DomainHealth: ensureArray(data.DomainHealth),
      DoneToday: ensureArray(data.DoneToday),
    }
  },
}

// Goals

export const goalsQuery = (domainId, status) => ({
  queryKey: ["goals", { domainId, status }],
  queryFn: async () => {
    const params = new URLSearchParams()
    if (domainId) params.set("domain_id", domainId)
    if (status) params.set("status", status)
    const data = await apiFetch(`/api/goals?${params}`)
    return ensureArray(data)
  },
})

export const goalDetailQuery = (id) => ({
  queryKey: ["goal", id],
  queryFn: () => apiFetch(`/api/goals/${id}`),
})

// Tasks

export const tasksQuery = (domainId, goalId, status, due) => ({
  queryKey: ["tasks", { domainId, goalId, status, due }],
  queryFn: async () => {
    const params = new URLSearchParams()
    if (domainId) params.set("domain_id", domainId)
    if (goalId) params.set("goal_id", goalId)
    if (status) params.set("status", status)
    if (due) params.set("due", due)
    const data = await apiFetch(`/api/tasks?${params}`)
    return ensureArray(data)
  },
})

// Notes

export const notesQuery = (domainId, goalId) => ({
  queryKey: ["notes", { domainId, goalId }],
  queryFn: async () => {
    const params = new URLSearchParams()
    if (domainId) params.set("domain_id", domainId)
    if (goalId) params.set("goal_id", goalId)
    const data = await apiFetch(`/api/notes?${params}`)
    return ensureArray(data)
  },
})

export const noteDetailQuery = (id) => ({
  queryKey: ["note", id],
  queryFn: () => apiFetch(`/api/notes/${id}`),
})

// Domains

export const domainsQuery = {
  queryKey: ["domains"],
  queryFn: async () => {
    const raw = await apiFetch("/api/domains")
    return ensureArray(raw)
      .map(normalizeDomain)
      .filter(Boolean)
  },
}

// Domain Health (for Notebooks)

export const domainHealthQuery = {
  queryKey: ["domains-health"],
  queryFn: async () => {
    const data = await apiFetch("/api/domains/health")
    return ensureArray(data)
  },
}

// Inbox

export const inboxQuery = {
  queryKey: ["inbox"],
  queryFn: async () => {
    const data = await apiFetch("/api/inbox")
    return ensureArray(data)
  },
}

export const inboxCountQuery = {
  queryKey: ["inbox-count"],
  queryFn: async () => {
    const items = await apiFetch("/api/inbox")
    return Array.isArray(items) ? items.filter(i => i.Processed === 0).length : 0
  },
  staleTime: 30000,
}

// Search

export const searchQuery = (q, filters) => ({
  queryKey: ["search", q, filters],
  queryFn: async () => {
    const params = new URLSearchParams()
    if (q) params.set("q", q)
    if (filters?.domain_id) params.set("domain_id", filters.domain_id)
    if (filters?.type) params.set("type", filters.type)
    if (filters?.status) params.set("status", filters.status)
    const data = await apiFetch(`/api/search?${params}`)
    return ensureArray(data)
      .map(normalizeSearchResult)
      .filter(Boolean)
  },
  enabled: !!q,
})

// Calendar

export const calendarQuery = {
  queryKey: ["calendar"],
  queryFn: async () => {
    const data = await apiFetch("/api/calendar/upcoming", { allow404: true, fallback: [] })
    return ensureArray(data)
      .map(normalizeCalendarEvent)
      .filter(Boolean)
  },
  staleTime: 5 * 60 * 1000,
}

// Mutations — Goals

export function createGoal(data) { return postJSON("/api/goals", data) }
export function updateGoal(id, data) { return putJSON(`/api/goals/${id}`, data) }
export function deleteGoal(id) { return apiFetch(`/api/goals/${id}`, { method: "DELETE" }) }

// Mutations — Tasks

export function createTask(data) { return postJSON("/api/tasks", data) }
export function updateTask(id, data) { return putJSON(`/api/tasks/${id}`, data) }
export function completeTask(id) { return putJSON(`/api/tasks/${id}/complete`, {}) }
export function reorderTask(id, position) { return putJSON(`/api/tasks/${id}/reorder`, { position }) }
export function setTaskFocus(id) { return putJSON(`/api/tasks/${id}/focus`, {}) }
export function clearTaskFocus() { return apiFetch(`/api/tasks/focus`, { method: "DELETE" }) }
export function deleteTask(id) { return apiFetch(`/api/tasks/${id}`, { method: "DELETE" }) }

// Mutations — Notes

export function createNote(data) { return postJSON("/api/notes", data) }
export function updateNote(id, data) { return putJSON(`/api/notes/${id}`, data) }
export function deleteNote(id) { return apiFetch(`/api/notes/${id}`, { method: "DELETE" }) }

// Mutations — Domains

export function createDomain(data) { return postJSON("/api/domains", data) }
export function updateDomain(id, data) {
  return apiFetch(`/api/domains/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}
export function archiveDomain(id) { return apiFetch(`/api/domains/${id}`, { method: "DELETE" }) }

// Mutations — Inbox

export function processInboxItem(id, data) { return putJSON(`/api/inbox/${id}/process`, data) }
export function archiveInboxCapture(id) { return apiFetch(`/api/inbox/${id}/archive`, { method: "POST" }) }
export function createCapture(text) { return postJSON("/api/capture", { text }) }

// Coder
export function dispatchCoderTask({ task, chain = "single", cwd = "" }) {
  return postJSON("/api/coder/dispatch", { task, chain, cwd })
}

export function fetchStepEvents(taskId, stepNum) {
  return apiFetch(`/api/coder/tasks/${taskId}/steps/${stepNum}/events`)
}
