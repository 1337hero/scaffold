import { useEffect, useState } from "preact/hooks";

function parseHash() {
  const hash = location.hash.replace(/^#\/?/, "") || "today";
  const parts = hash.split("/");
  return { route: parts[0] || "today", param: parts[1] || null };
}

function navigate(path) {
  location.hash = path;
}

function useRoute() {
  const [route, setRoute] = useState(parseHash);

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  return route;
}

export { navigate, useRoute };
