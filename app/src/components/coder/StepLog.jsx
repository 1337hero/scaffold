import { useState } from "preact/hooks"

const badgeStyle = (ev) => {
  if (ev.type === "tool_use") {
    const tool = (ev.tool || "").toLowerCase()
    if (tool === "bash")  return { label: "cmd",   cls: "tool-badge-shell" }
    if (tool === "write") return { label: "write", cls: "tool-badge-write" }
    if (tool === "read")  return { label: "read",  cls: "tool-badge-read" }
    if (tool === "edit" || tool === "multiedit") return { label: "edit", cls: "tool-badge-write" }
    if (tool === "grep" || tool === "find" || tool === "ls") return { label: tool, cls: "tool-badge-shell" }
    return { label: "tool", cls: "bg-black/5 text-app-muted" }
  }
  if (ev.type === "tool_result") return { label: "done", cls: "tool-badge-result" }
  if (ev.type === "result") return { label: "done", cls: "tool-badge-result" }
  if (ev.type === "error") return { label: "error", cls: "tool-badge-error" }
  return null
}

const EventRow = ({ ev, isLatest }) => {
  const badge = badgeStyle(ev)

  if (ev.type === "assistant") {
    return (
      <div class="text-[11px] text-app-muted leading-relaxed pl-2 border-l border-app-border my-0.5">
        {ev.text}
      </div>
    )
  }

  return (
    <div class="flex items-baseline gap-2 py-0.5 border-b border-black/[0.025] last:border-b-0 pb-[3px]">
      {badge && (
        <span class={`shrink-0 inline-block px-1.5 py-px rounded text-[9px] font-bold font-mono uppercase tracking-wide ${badge.cls}`}>
          {badge.label}
        </span>
      )}
      <span class={`text-[11px] font-mono flex-1 ${isLatest ? "text-app-ink" : "text-app-muted"}`}>
        {ev.input || ev.text || ev.result || ""}
      </span>
    </div>
  )
}

function stitchEvents(events) {
  const result = []
  let accum = null
  for (const ev of events) {
    if (ev.type === "assistant") {
      if (accum) {
        accum.text += ev.text
      } else {
        accum = { ...ev, text: ev.text || "" }
      }
    } else {
      if (accum) {
        result.push(accum)
        accum = null
      }
      result.push(ev)
    }
  }
  if (accum) result.push(accum)
  return result
}

const StepLog = ({ steps, stepLogs }) => {
  const [open, setOpen] = useState(false)

  const totalEvents = steps.reduce((n, s) => n + (stepLogs[s.name] || []).length, 0)
  const totalTools = steps.reduce((n, s) => {
    return n + (stepLogs[s.name] || []).filter((e) => e.type === "tool_use").length
  }, 0)

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        class="flex items-center gap-1.5 bg-transparent border-none p-0 cursor-pointer text-app-muted font-mono text-[10px] uppercase tracking-wide opacity-45 hover:opacity-75 transition-opacity font-medium"
      >
        <svg class={`log-arrow ${open ? "open" : ""}`} width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6" /></svg>
        {steps.length} step{steps.length !== 1 ? "s" : ""}
        {" · "}
        {totalTools} tool call{totalTools !== 1 ? "s" : ""}
        {" · "}
        {open ? "collapse" : "expand"} log
      </button>

      {open && (
        <div class="flex flex-col gap-0.5 mt-2.5">
          {steps.map((step) => {
            const events = stepLogs[step.name] || []
            if (!events.length) return null
            const stitched = stitchEvents(events)
            return (
              <div key={step.name}>
                <div class="font-mono text-[9px] uppercase tracking-[0.08em] text-app-muted opacity-40 mt-2 mb-1 first:mt-0">
                  {step.name}
                </div>
                {stitched.map((ev, i) => (
                  <EventRow key={i} ev={ev} isLatest={i === stitched.length - 1} />
                ))}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

export default StepLog
