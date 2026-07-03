import { SURFACES } from "@/constants/surfaces.js";
import { useSurface } from "@/hooks/useSurface.jsx";
import { cn } from "@/lib/utils.js";

const SurfaceToggle = () => {
  const { surface, setSurface } = useSurface();

  return (
    <div
      class="fixed top-4 right-4 z-40 flex items-center gap-1 p-1 rounded-full bg-card-bg border border-app-border shadow-sm"
      role="group"
      aria-label="Surface"
    >
      {Object.values(SURFACES).map((s) => {
        const active = surface === s.id;
        return (
          <button
            key={s.id}
            type="button"
            onClick={() => setSurface(s.id)}
            class={cn(
              "flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium transition-all duration-200",
              active ? s.activeClass : "text-app-muted hover:text-app-ink",
            )}
            aria-pressed={active}
          >
            <s.icon size={16} class="shrink-0" />
            <span class={cn(!active && "hidden sm:inline")}>{s.short}</span>
          </button>
        );
      })}
    </div>
  );
};

export default SurfaceToggle;
