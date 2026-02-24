const formatElapsed = (secs) => {
  const s = Math.floor(secs)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const rem = s % 60
  return `${m}m ${rem.toString().padStart(2, "0")}s`
}

const StepPipeline = ({ steps }) => {
  return (
    <div class="flex items-center gap-1 flex-wrap my-3">
      {steps.map((step, i) => (
        <>
          {i > 0 && (
            <span key={`arrow-${i}`} class="text-[#5A4F42] text-xs">→</span>
          )}
          <StepBadge key={step.name} step={step} />
        </>
      ))}
    </div>
  )
}

const StepBadge = ({ step }) => {
  const styles = {
    done:    "bg-[#1E3A2E] text-[#4ADE80] border-[#2A5A3E]",
    running: "bg-[#2E2318] text-[#C47D3A] border-[#C47D3A]/40",
    failed:  "bg-[#3A1E1E] text-[#F87171] border-[#5A2E2E]",
    pending: "bg-transparent text-[#5A4F42] border-[#3A3228]",
  }

  const icons = {
    done:    "✓",
    running: "●",
    failed:  "✗",
    pending: "○",
  }

  const status = step.status || "pending"
  const cls = styles[status] || styles.pending

  return (
    <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono border ${cls}`}>
      <span class={status === "running" ? "animate-pulse" : ""}>{icons[status]}</span>
      {step.name}
      {(status === "done" || status === "failed") && step.elapsed_s > 0 && (
        <span class="opacity-50 ml-1">{formatElapsed(step.elapsed_s)}</span>
      )}
    </span>
  )
}

export default StepPipeline
