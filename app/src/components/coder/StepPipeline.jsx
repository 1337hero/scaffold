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
  done:    "bg-[#5A9E6F]/12 text-[#5A9E6F]",
  running: "bg-[#C47D3A]/12 text-[#C47D3A] step-icon-active",
  failed:  "bg-[#C4617A]/10 text-[#C4617A]",
  pending: "bg-black/4 text-app-muted",
}

const nameCls = {
  done:    "text-[#5A9E6F]",
  running: "text-[#C47D3A] font-medium",
  failed:  "text-[#C4617A]",
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
