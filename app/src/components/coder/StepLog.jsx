import { useState } from "preact/hooks"

const eventBadge = (ev) => {
  if (ev.type === "tool_use") {
    const tool = (ev.tool || "").toLowerCase()
    if (tool === "bash")  return { label: "cmd",   cls: "bg-[#1A2E1A] text-[#4ADE80] border-[#2A4A2A]" }
    if (tool === "write") return { label: "write",  cls: "bg-[#1A1A2E] text-[#818CF8] border-[#2A2A4A]" }
    if (tool === "read")  return { label: "read",   cls: "bg-[#1A2A2E] text-[#38BDF8] border-[#2A3A4A]" }
    if (tool === "edit" || tool === "multiedit") return { label: "edit", cls: "bg-[#2E2818] text-[#FBD38D] border-[#4A3E28]" }
    return { label: "tool", cls: "bg-[#2A2A2A] text-[#9C8E7A] border-[#3A3A3A]" }
  }
  if (ev.type === "tool_result") return { label: "done", cls: "bg-[#1E3A2E] text-[#4ADE80] border-[#2A5A3E]" }
  if (ev.type === "result") return { label: "done",  cls: "bg-[#1E3A2E] text-[#4ADE80] border-[#2A5A3E]" }
  if (ev.type === "error")  return { label: "error", cls: "bg-[#3A1E1E] text-[#F87171] border-[#5A2E2E]" }
  return null
}

const EventRow = ({ ev }) => {
  const badge = eventBadge(ev)

  if (ev.type === "assistant") {
    return (
      <div class="text-[11px] text-[#6B5F52] leading-relaxed pl-2 border-l border-[#2A2318] my-0.5">
        {ev.text}
      </div>
    )
  }

  return (
    <div class="flex items-start gap-2 py-0.5">
      {badge && (
        <span class={`shrink-0 inline-block px-1.5 py-0 rounded text-[10px] font-mono border mt-0.5 ${badge.cls}`}>
          {badge.label}
        </span>
      )}
      <span class="text-[11px] font-mono text-[#9C8E7A] truncate">
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

const StepLog = ({ step, events = [] }) => {
  const [open, setOpen] = useState(false)

  const count = events.length
  const statusColor = {
    done:    "text-[#4ADE80]",
    running: "text-[#C47D3A]",
    failed:  "text-[#F87171]",
    pending: "text-[#5A4F42]",
  }[step.status] || "text-[#5A4F42]"

  return (
    <div class="mt-2 border border-[#2A2318] rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        class="w-full flex items-center justify-between px-3 py-2 text-left hover:bg-white/[0.02] transition-colors"
      >
        <span class={`text-[11px] font-mono ${statusColor}`}>
          {step.name}: {count} event{count !== 1 ? "s" : ""}
        </span>
        <span class="text-[10px] text-[#5A4F42]">{open ? "▾" : "▸"}</span>
      </button>

      {open && count > 0 && (
        <div class="px-3 pb-3 space-y-0.5 border-t border-[#2A2318]">
          {stitchEvents(events).map((ev, i) => (
            <EventRow key={i} ev={ev} />
          ))}
        </div>
      )}
    </div>
  )
}

export default StepLog
