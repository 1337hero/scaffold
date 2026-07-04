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
    task: 'var(--color-type-task)',
    note: 'var(--color-type-note)',
  }
  return map[type] || DEFAULT_DOMAIN
}
