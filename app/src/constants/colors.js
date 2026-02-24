// Domain name → CSS variable mapping
const DOMAIN_VAR = {
  'work/business': 'var(--color-domain-work)',
  'business': 'var(--color-domain-work)',
  'personal projects': 'var(--color-domain-projects)',
  'projects': 'var(--color-domain-projects)',
  'homelife': 'var(--color-domain-homelife)',
  'home': 'var(--color-domain-homelife)',
  'personal development': 'var(--color-domain-development)',
  'health': 'var(--color-domain-development)',
  'relationships': 'var(--color-domain-relationships)',
  'finances': 'var(--color-domain-finances)',
  'hobbies': 'var(--color-domain-hobbies)',
}

const DEFAULT_DOMAIN = 'var(--color-domain-default)'

// Returns a var() string for use in inline styles.
// Accepts domain name string OR domain object with .Name/.Color.
export function domainColor(nameOrDomain) {
  if (!nameOrDomain) return DEFAULT_DOMAIN
  if (typeof nameOrDomain === 'object') {
    if (nameOrDomain.Color?.Valid) return nameOrDomain.Color.String
    return DOMAIN_VAR[nameOrDomain.Name?.toLowerCase()] || DEFAULT_DOMAIN
  }
  return DOMAIN_VAR[nameOrDomain.toLowerCase()] || DEFAULT_DOMAIN
}

// Domain name → color-mix bg string for badges
export function domainBg(nameOrDomain) {
  const color = domainColor(nameOrDomain)
  return `color-mix(in srgb, ${color} 12%, transparent)`
}

// Capture type colors
export function typeColor(type) {
  const map = {
    goal: 'var(--color-type-goal)',
    task: 'var(--color-type-task)',
    note: 'var(--color-type-note)',
  }
  return map[type] || DEFAULT_DOMAIN
}

// Drift state → CSS class
export function driftClass(state) {
  const s = (state || 'active').toLowerCase()
  const map = { active: 'drift-active', drifting: 'drift-drifting', neglected: 'drift-neglected', cold: 'drift-cold', overactive: 'drift-overactive' }
  return map[s] || 'drift-active'
}

// Inbox group dot colors
export const GROUP_DOTS = {
  tasks: 'var(--color-group-tasks)',
  goals: 'var(--color-group-goals)',
  notes: 'var(--color-group-notes)',
}

// Legacy exports (use Tailwind class names, not hex)
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
  article: 'bg-cyan/10 text-cyan',
  ref: 'bg-cyan/10 text-cyan',
}
