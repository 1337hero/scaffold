const stateColors = {
  active: "text-green",
  drifting: "text-amber",
  neglected: "text-red",
  cold: "text-text-muted",
  overactive: "text-purple",
}

export function DriftLabel({ state, label }) {
  const color = stateColors[state] ?? "text-text-dim"
  return <span class={`text-xs font-medium ${color}`}>{label}</span>
}
