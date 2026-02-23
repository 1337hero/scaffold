const DOMAIN_COLORS = {
  'Work/Business': { bg: 'rgba(91,141,184,0.12)', text: '#5B8DB8' },
  'Personal Projects': { bg: 'rgba(139,107,177,0.12)', text: '#8B6BB1' },
  'Homelife': { bg: 'rgba(90,158,111,0.12)', text: '#5A9E6F' },
  'Relationships': { bg: 'rgba(196,97,122,0.12)', text: '#C4617A' },
  'Personal Development': { bg: 'rgba(196,125,58,0.12)', text: '#C47D3A' },
  'Finances': { bg: 'rgba(61,158,158,0.12)', text: '#3D9E9E' },
  'Hobbies': { bg: 'rgba(196,102,58,0.12)', text: '#C4663A' },
}

const DomainBadge = ({ name }) => {
  const colors = DOMAIN_COLORS[name] || { bg: 'rgba(156,142,122,0.12)', text: '#9C8E7A' }
  return (
    <span class="domain-badge" style={{ background: colors.bg, color: colors.text, border: '1px solid rgba(0,0,0,0.06)' }}>
      {name}
    </span>
  )
}

export default DomainBadge
