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

// --- v2 normalizers (clean client shapes from Go wire format) ---

function nullableField(field) {
  if (!field || typeof field !== "object" || !field.Valid) return null
  return field.String ?? field.Int64 ?? field.Float64 ?? null
}

function normalizeTask(t) {
  if (!t || typeof t !== "object") return null
  return {
    id: t.ID || "",
    title: t.Title || "",
    domainName: t.DomainName || "",
    domainId: nullableField(t.DomainID),
    dueDate: nullableField(t.DueDate),
    priority: t.Priority || "normal",
    status: t.Status || "pending",
    surface: t.Surface || "life",
    context: nullableField(t.Context) || "",
    microSteps: nullableField(t.MicroSteps) || "",
    recurring: nullableField(t.Recurring),
    projectId: nullableField(t.ProjectID),
    reminderAt: nullableField(t.ReminderAt),
    top3Position: nullableField(t.Top3Position),
    daysOverdue: t.DaysOverdue ?? 0,
  }
}

function normalizeSlippingProject(p) {
  if (!p || typeof p !== "object") return null
  return {
    id: p.ID || "",
    name: p.Name || "",
    type: p.Type || "project",
    surface: p.Surface || "life",
    status: p.Status || "active",
    lastActivityAt: nullableField(p.LastActivityAt),
  }
}

function normalizeSlippingPerson(p) {
  if (!p || typeof p !== "object") return null
  return {
    id: p.ID || "",
    name: p.Name || "",
    relationship: nullableField(p.Relationship),
    lastInteractionAt: nullableField(p.LastInteractionAt),
    contactCadenceDays: nullableField(p.ContactCadenceDays) ?? 90,
  }
}

// Today

export const todayQuery = (surface) => ({
  queryKey: ["today", surface],
  queryFn: async () => {
    const data = await apiFetch(`/api/today?surface=${surface}`)
    const slipping = data?.slipping ?? {}
    return {
      top3: ensureArray(data?.top3).map(normalizeTask).filter(Boolean),
      calendar: ensureArray(data?.calendar),
      slipping: {
        projects: ensureArray(slipping.projects).map(normalizeSlippingProject).filter(Boolean),
        tasks: ensureArray(slipping.tasks).map(normalizeTask).filter(Boolean),
        people: ensureArray(slipping.people).map(normalizeSlippingPerson).filter(Boolean),
        areas: ensureArray(slipping.areas).map(normalizeSlippingProject).filter(Boolean),
      },
      notifications: ensureArray(data?.notifications),
    }
  },
  staleTime: 60 * 1000,
})

export const top3CandidatesQuery = (surface) => ({
  queryKey: ["top3-candidates", surface],
  queryFn: async () => {
    const data = await apiFetch(`/api/tasks?surface=${surface}&top3=false`)
    return ensureArray(data).map(normalizeTask).filter(Boolean)
  },
})

export function setTop3(taskIds) { return putJSON("/api/today/top3", taskIds) }

// Tasks page

function normalizeProjectFull(p) {
  if (!p || typeof p !== "object") return null
  return {
    id: p.ID || "",
    name: p.Name || "",
    type: p.Type || "project",
    surface: p.Surface || "life",
    status: p.Status || "active",
    domainId: nullableField(p.DomainID),
    startDate: nullableField(p.StartDate),
    endDate: nullableField(p.EndDate),
    description: nullableField(p.Description) || "",
    lastActivityAt: nullableField(p.LastActivityAt),
    lastResetAt: nullableField(p.LastResetAt),
  }
}

function normalizeNote(n) {
  if (!n || typeof n !== "object") return null
  return {
    id: n.ID || "",
    title: n.Title || "",
    content: nullableField(n.Content) || "",
    kind: n.Kind || "note",
    tags: nullableField(n.Tags) || "",
    source: nullableField(n.Source) || "",
    personId: nullableField(n.PersonID),
    taskId: nullableField(n.TaskID),
    reviewAt: nullableField(n.ReviewAt),
    flagForReview: Boolean(n.FlagForReview),
    projectId: nullableField(n.ProjectID),
    createdAt: n.CreatedAt || "",
    updatedAt: nullableField(n.UpdatedAt),
  }
}

function normalizeKids(raw) {
  if (Array.isArray(raw)) return raw
  const value = nullableField(raw) ?? raw
  if (!value || typeof value !== "string") return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function normalizePerson(p) {
  if (!p || typeof p !== "object") return null
  return {
    id: p.ID || p.id || "",
    name: p.Name || p.name || "",
    surface: p.Surface || p.surface || "life",
    domainId: nullableField(p.DomainID ?? p.domain_id),
    relationship: nullableField(p.Relationship ?? p.relationship),
    birthday: nullableField(p.Birthday ?? p.birthday),
    anniversary: nullableField(p.Anniversary ?? p.anniversary),
    spouse: nullableField(p.Spouse ?? p.spouse),
    kids: normalizeKids(p.Kids ?? p.kids),
    notes: nullableField(p.Notes ?? p.notes) || "",
    lastInteractionAt: nullableField(p.LastInteractionAt ?? p.last_interaction_at),
    contactCadenceDays: nullableField(p.ContactCadenceDays ?? p.contact_cadence_days) ?? 90,
    createdAt: p.CreatedAt || p.created_at || "",
    updatedAt: p.UpdatedAt || p.updated_at || "",
  }
}

function normalizeInteraction(i) {
  if (!i || typeof i !== "object") return null
  return {
    id: i.ID || i.id || "",
    personId: i.PersonID || i.person_id || "",
    date: i.Date || i.date || "",
    summary: i.Summary || i.summary || "",
    followUp: nullableField(i.FollowUp ?? i.follow_up),
    followUpDate: nullableField(i.FollowUpDate ?? i.follow_up_date),
    createdAt: i.CreatedAt || i.created_at || "",
  }
}

function normalizeBirthdayHit(hit) {
  if (!hit || typeof hit !== "object") return null
  return {
    personId: hit.person_id || hit.PersonID || "",
    name: hit.name || hit.Name || "",
    kind: hit.kind || hit.Kind || "self",
    date: hit.date || hit.Date || "",
    daysUntil: hit.days_until ?? hit.DaysUntil ?? 0,
    urgency: hit.urgency || hit.Urgency || "upcoming",
    relationship: hit.relationship || hit.Relationship || "",
  }
}

export const tasksListQuery = (surface, status) => ({
  queryKey: ["tasks-list", surface, status],
  queryFn: async () => {
    const data = await apiFetch(`/api/tasks?surface=${surface}&status=${status}`)
    return ensureArray(data).map(normalizeTask).filter(Boolean)
  },
})

export const top3IdsQuery = {
  queryKey: ["top3-ids"],
  queryFn: async () => {
    const data = await apiFetch("/api/tasks?top3=true")
    return ensureArray(data)
      .map(normalizeTask)
      .filter(Boolean)
      .sort((a, b) => (a.top3Position ?? 0) - (b.top3Position ?? 0))
      .map((t) => t.id)
  },
}

export const projectsListQuery = (surface = "") => ({
  queryKey: ["projects-list", surface],
  queryFn: async () => {
    const params = surface ? `?surface=${surface}` : ""
    const data = await apiFetch(`/api/projects${params}`)
    return ensureArray(data).map(normalizeProjectFull).filter(Boolean)
  },
})

export const taskNotesQuery = (taskId) => ({
  queryKey: ["task-notes", taskId],
  queryFn: async () => {
    const data = await apiFetch(`/api/notes?task_id=${taskId}`)
    return ensureArray(data).map(normalizeNote).filter(Boolean)
  },
})

// Projects page

function normalizeMilestone(m) {
  if (!m || typeof m !== "object") return null
  return {
    id: m.ID || "",
    projectId: m.ProjectID || "",
    title: m.Title || "",
    position: m.Position ?? 0,
    completed: Boolean(m.Completed),
  }
}

function normalizeChecklist(c) {
  if (!c || typeof c !== "object") return null
  let items = []
  try {
    const parsed = JSON.parse(c.Items || "[]")
    if (Array.isArray(parsed)) items = parsed
  } catch { /* malformed items stay empty */ }
  return {
    id: c.ID || "",
    title: c.Title || "",
    items,
    isTemplate: Boolean(c.IsTemplate),
  }
}

function normalizeActivity(a) {
  if (!a || typeof a !== "object") return null
  return {
    id: a.ID || "",
    description: a.Description || "",
    hours: nullableField(a.Hours),
    createdAt: a.CreatedAt || "",
  }
}

export const projectDetailQuery = (id) => ({
  queryKey: ["project-detail", id],
  queryFn: async () => {
    const data = await apiFetch(`/api/projects/${id}`)
    return {
      project: normalizeProjectFull(data?.project),
      milestones: ensureArray(data?.milestones).map(normalizeMilestone).filter(Boolean),
      milestoneCompleted: data?.milestone_completed ?? 0,
      milestoneTotal: data?.milestone_total ?? 0,
      checklists: ensureArray(data?.checklists).map(normalizeChecklist).filter(Boolean),
      recentActivity: ensureArray(data?.recent_activity).map(normalizeActivity).filter(Boolean),
    }
  },
})

export const projectTasksQuery = (projectId) => ({
  queryKey: ["project-tasks", projectId],
  queryFn: async () => {
    const data = await apiFetch(`/api/tasks?project_id=${projectId}`)
    return ensureArray(data).map(normalizeTask).filter(Boolean)
  },
})

export const projectActivityQuery = (projectId) => ({
  queryKey: ["project-activity", projectId],
  queryFn: async () => {
    const data = await apiFetch(`/api/projects/${projectId}/activity`)
    return ensureArray(data).map(normalizeActivity).filter(Boolean)
  },
})

export const checklistTemplatesQuery = {
  queryKey: ["checklist-templates"],
  queryFn: async () => {
    const data = await apiFetch("/api/checklists/templates")
    return ensureArray(data).map(normalizeChecklist).filter(Boolean)
  },
}

// Mutations — Projects/Milestones/Checklists/Activity

export function createProject(data) { return postJSON("/api/projects", data) }
export function patchProject(id, data) {
  return apiFetch(`/api/projects/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}
export function archiveProject(id) { return apiFetch(`/api/projects/${id}`, { method: "DELETE" }) }

export function createMilestone(projectId, data) { return postJSON(`/api/projects/${projectId}/milestones`, data) }
export function patchMilestone(id, data) {
  return apiFetch(`/api/milestones/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}
export function deleteMilestone(id) { return apiFetch(`/api/milestones/${id}`, { method: "DELETE" }) }

export function createChecklist(projectId, data) { return postJSON(`/api/projects/${projectId}/checklists`, data) }
export function patchChecklist(id, data) {
  return apiFetch(`/api/checklists/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}
export function cloneChecklist(projectId, templateId) {
  return postJSON(`/api/projects/${projectId}/checklists/clone`, { template_id: templateId })
}
export function createChecklistTemplate(data) { return postJSON("/api/checklists/templates", data) }

export function logActivity(projectId, data) { return postJSON(`/api/projects/${projectId}/activity`, data) }

// People page

export const peopleListQuery = (surface) => ({
  queryKey: ["people-list", surface],
  queryFn: async () => {
    const params = surface ? `?surface=${surface}` : ""
    const data = await apiFetch(`/api/people${params}`)
    return ensureArray(data).map(normalizePerson).filter(Boolean)
  },
})

export const personDetailQuery = (id) => ({
  queryKey: ["person-detail", id],
  queryFn: async () => normalizePerson(await apiFetch(`/api/people/${id}`)),
})

export const personInteractionsQuery = (id) => ({
  queryKey: ["person-interactions", id],
  queryFn: async () => {
    const data = await apiFetch(`/api/people/${id}/interactions`)
    return ensureArray(data).map(normalizeInteraction).filter(Boolean)
  },
})

export const personNotesQuery = (personId) => ({
  queryKey: ["person-notes", personId],
  queryFn: async () => {
    const data = await apiFetch(`/api/notes?person_id=${personId}`)
    return ensureArray(data).map(normalizeNote).filter(Boolean)
  },
})

export const birthdaysQuery = (days = 7) => ({
  queryKey: ["birthdays", days],
  queryFn: async () => {
    const data = await apiFetch(`/api/people/birthdays?days=${days}`)
    return ensureArray(data).map(normalizeBirthdayHit).filter(Boolean)
  },
})

export function createPerson(data) { return postJSON("/api/people", data) }
export function patchPerson(id, data) {
  return apiFetch(`/api/people/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}
export function deletePerson(id) { return apiFetch(`/api/people/${id}`, { method: "DELETE" }) }
export function logInteraction(personId, data) { return postJSON(`/api/people/${personId}/interactions`, data) }

// Library page

export const libraryNotesQuery = (filters) => ({
  queryKey: ["library-notes", filters],
  queryFn: async () => {
    const params = new URLSearchParams()
    if (filters?.kind) params.set("kind", filters.kind)
    if (filters?.surface) params.set("surface", filters.surface)
    if (filters?.tags) params.set("tags", filters.tags)
    if (filters?.source) params.set("source", filters.source)
    if (filters?.flagForReview != null && filters.flagForReview !== "") {
      params.set("flag_for_review", filters.flagForReview)
    }
    if (filters?.q) params.set("q", filters.q)
    const data = await apiFetch(`/api/notes?${params}`)
    return ensureArray(data).map(normalizeNote).filter(Boolean)
  },
})
