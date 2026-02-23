import { cn } from "@/lib/utils.js"

const HealthBar = ({ value, className, color }) => {
  const pct = Math.round(value * 100)
  const colorClass = color ? null : (pct >= 70 ? 'bg-green' : pct >= 30 ? 'bg-amber' : 'bg-red')

  return (
    <div class={cn("health-bar", className)}>
      <div
        class={cn("health-bar-fill transition-[width] duration-700 ease-out", colorClass)}
        style={{ width: `${pct}%`, ...(color ? { backgroundColor: color } : {}) }}
      />
    </div>
  )
}

export default HealthBar
