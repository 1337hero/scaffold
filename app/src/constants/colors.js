export const projectColors = {
  amber: { text: 'text-amber', bg: 'bg-amber-dim', dot: 'bg-amber' },
  blue: { text: 'text-blue', bg: 'bg-blue-dim', dot: 'bg-blue' },
  purple: { text: 'text-purple', bg: 'bg-purple-dim', dot: 'bg-purple' },
  green: { text: 'text-green', bg: 'bg-green-dim', dot: 'bg-green' },
  cyan: { text: 'text-cyan', bg: 'bg-cyan/10', dot: 'bg-cyan' },
  red: { text: 'text-red', bg: 'bg-red-dim', dot: 'bg-red' },
}

export const colorClass = (color, variant = 'text') =>
  projectColors[color]?.[variant] ?? 'text-text-dim'

export const typeStyles = {
  task: 'bg-amber-dim text-amber',
  link: 'bg-blue-dim text-blue',
  idea: 'bg-purple-dim text-purple',
  video: 'bg-red-dim text-red',
  note: 'bg-green-dim text-green',
  article: 'bg-[rgba(6,182,212,0.1)] text-cyan',
  ref: 'bg-[rgba(6,182,212,0.1)] text-cyan',
}
