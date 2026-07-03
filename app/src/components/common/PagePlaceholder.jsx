import { SURFACES } from "@/constants/surfaces.js";
import { useSurface } from "@/hooks/useSurface.jsx";

const PagePlaceholder = ({ title, description }) => {
  const { surface } = useSurface();

  return (
    <div>
      <h1 class="font-serif italic text-3xl font-semibold tracking-tight mb-2">{title}</h1>
      <p class="text-app-muted">
        {description} Showing {SURFACES[surface].label}.
      </p>
    </div>
  );
};

export default PagePlaceholder;
