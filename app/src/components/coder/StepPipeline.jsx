const formatElapsed = (secs) => {
  const s = Math.floor(secs)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const rem = s % 60
  return `${m}m ${rem.toString().padStart(2, "0")}s`
}

const StepPipeline = ({ steps }) => {
  return (
    <div class="flex items-center gap-0 mb-4">
      {steps.map((step, i) => (
        <>
          {i > 0 && (
            <span key={`arrow-${i}`} class="text-app-border text-xs mx-2 opacity-80">→</span>
          )}
          <StepNode key={step.name} step={step} />
        </>
      ))}
    </div>
  )
}

const iconCls = {
  done:    "step-done",
  running: "step-running step-icon-active",
  failed:  "step-failed",
  pending: "step-pending",
}

const nameCls = {
  done:    "text-status-done",
  running: "text-status-running font-medium",
  failed:  "text-status-failed",
  pending: "text-app-muted opacity-40",
}

const icons = {
  done:    "✓",
  running: "⟳",
  failed:  "✗",
  pending: "○",
}

const StepNode = ({ step }) => {
  const status = step.status || "pending"

  return (
    <div class="flex items-center gap-1.5 font-mono text-[11px]">
      <div class={`w-[22px] h-[22px] rounded-full flex items-center justify-center text-[9px] font-bold shrink-0 ${iconCls[status] || iconCls.pending}`}>
        {icons[status]}
      </div>
      <span class={`text-[10px] ${nameCls[status] || nameCls.pending}`}>
        {step.name}
      </span>
    </div>
  )
}

export default StepPipeline
