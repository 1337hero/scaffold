const stateColors = {
  active: "text-green",
  drifting: "text-amber",
  neglected: "text-red",
  cold: "text-text-muted",
  overactive: "text-purple",
}

const DriftLabel = ({ state, label }) => {
  const color = stateColors[state] ?? "text-text-dim"
  return <span class={`text-xs font-medium ${color}`}>{label}</span>
}

export default DriftLabel
