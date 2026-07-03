import { SURFACES } from "@/constants/surfaces.js";
import { createContext } from "preact";
import { useContext, useState } from "preact/hooks";

const STORAGE_KEY = "scaffold-surface";

// Work hours default to BusinessOS; evenings and mornings to LifeOS.
function defaultSurface() {
  const hour = new Date().getHours();
  return hour >= 8 && hour < 18 ? "business" : "life";
}

// sessionStorage, not localStorage: an explicit choice survives reloads but
// resets when the session ends, so the time-of-day default applies fresh
// each day instead of being pinned forever.
function initialSurface() {
  const stored = sessionStorage.getItem(STORAGE_KEY);
  return stored in SURFACES ? stored : defaultSurface();
}

const SurfaceContext = createContext(null);

const SurfaceProvider = ({ children }) => {
  const [surface, setSurfaceState] = useState(initialSurface);

  const setSurface = (next) => {
    sessionStorage.setItem(STORAGE_KEY, next);
    setSurfaceState(next);
  };

  const toggle = () =>
    setSurfaceState((prev) => {
      const next = prev === "life" ? "business" : "life";
      sessionStorage.setItem(STORAGE_KEY, next);
      return next;
    });

  return (
    <SurfaceContext.Provider value={{ surface, setSurface, toggle }}>
      {children}
    </SurfaceContext.Provider>
  );
};

const useSurface = () => {
  const ctx = useContext(SurfaceContext);
  if (!ctx) throw new Error("useSurface must be used within SurfaceProvider");
  return ctx;
};

export { SurfaceProvider, useSurface };
