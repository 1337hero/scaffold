import { cva } from "class-variance-authority"
import { cn } from "@/lib/utils.js"

const checkboxVariants = cva(
  "shrink-0 cursor-pointer transition-all flex items-center justify-center",
  {
    variants: {
      size: {
        sm: "w-4 h-4 rounded-sm border-[1.5px]",
        md: "w-[22px] h-[22px] rounded-md border-2",
      },
      checked: {
        true: "border-green bg-green-dim",
        false: "border-border-light bg-transparent hover:border-green hover:bg-green-dim",
      },
    },
    defaultVariants: { size: "md", checked: false },
  }
)

const checkSizes = { sm: "text-[0.55rem]", md: "text-[0.75rem]" }

export function Checkbox({ checked, onChange, size = "md" }) {
  return (
    <button
      onClick={onChange}
      class={cn(checkboxVariants({ size, checked }))}
      aria-label={checked ? "Mark incomplete" : "Mark complete"}
    >
      {checked && <span class={cn("text-green font-bold", checkSizes[size])}>{"\u2713"}</span>}
    </button>
  )
}
