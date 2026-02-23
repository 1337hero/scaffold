import { cva } from "class-variance-authority"
import { cn } from "@/lib/utils.js"

const checkboxVariants = cva(
  "shrink-0 cursor-pointer transition-all flex items-center justify-center focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber/70",
  {
    variants: {
      size: {
        sm: "w-[18px] h-[18px] rounded-sm border-[1.5px]",
        md: "w-6 h-6 rounded-md border-2",
      },
      checked: {
        true: "border-green bg-green-dim",
        false: "border-border-light bg-transparent hover:border-green hover:bg-green-dim",
      },
    },
    defaultVariants: { size: "md", checked: false },
  }
)

const checkSizes = { sm: "text-[0.62rem]", md: "text-[0.82rem]" }

const Checkbox = ({ checked, onChange, size = "md" }) => {
  return (
    <button
      type="button"
      onClick={onChange}
      class={cn(checkboxVariants({ size, checked }))}
      aria-label={checked ? "Mark incomplete" : "Mark complete"}
    >
      {checked && <span class={cn("text-green font-bold", checkSizes[size])}>{"\u2713"}</span>}
    </button>
  )
}

export default Checkbox
