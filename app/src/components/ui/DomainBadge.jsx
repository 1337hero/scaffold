import { domainColor, domainBg } from "@/constants/colors.js"

const DomainBadge = ({ name }) => {
  return (
    <span class="domain-badge" style={{ background: domainBg(name), color: domainColor(name), border: '1px solid rgba(0,0,0,0.06)' }}>
      {name}
    </span>
  )
}

export default DomainBadge
