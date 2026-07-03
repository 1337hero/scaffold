import MobileBar from "@/components/MobileBar.jsx";
import Sidebar from "@/components/Sidebar.jsx";
import SurfaceToggle from "@/components/SurfaceToggle.jsx";
import { useKeyboard } from "@/hooks/useKeyboard.js";
import { navigate, useRoute } from "@/hooks/useRoute.js";
import { SurfaceProvider, useSurface } from "@/hooks/useSurface.jsx";
import Area from "@/pages/Area.jsx";
import Areas from "@/pages/Areas.jsx";
import Library from "@/pages/Library.jsx";
import Login from "@/pages/Login.jsx";
import People from "@/pages/People.jsx";
import Projects from "@/pages/Projects.jsx";
import Search from "@/pages/Search.jsx";
import Tasks from "@/pages/Tasks.jsx";
import Today from "@/pages/Today.jsx";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "preact/hooks";

function RouteView({ route, param }) {
  switch (route) {
    case "today":
      return <Today />;
    case "tasks":
      return <Tasks />;
    case "projects":
      return <Projects projectId={param} />;
    case "people":
      return <People />;
    case "library":
      return <Library />;
    case "domains":
      return param ? (
        <Area domainId={param} onBack={() => navigate("/domains")} />
      ) : (
        <Areas onOpenArea={(id) => navigate(`/domains/${id}`)} />
      );
    case "search":
      return <Search />;
    default:
      return <Today />;
  }
}

const routeTitles = {
  today: "Today",
  tasks: "Tasks",
  projects: "Projects",
  people: "People",
  library: "Library",
  domains: "Domains",
  search: "Search",
};

function AuthenticatedShell() {
  const { route, param } = useRoute();
  const queryClient = useQueryClient();
  const { toggle } = useSurface();

  useKeyboard([{ key: ".", meta: true, action: toggle }]);

  useEffect(() => {
    document.title = `${routeTitles[route] ?? "Today"} — Scaffold`;
  }, [route]);

  const handleLogout = async () => {
    try {
      await fetch("/api/logout", { method: "POST", credentials: "include" });
    } finally {
      // Drop every cached query — no stale personal data after logout.
      queryClient.clear();
      queryClient.setQueryData(["auth"], false);
    }
  };

  return (
    <div class="min-h-screen pb-24 lg:pb-0 lg:pl-64">
      <Sidebar activeRoute={route} onLogout={handleLogout} />

      <SurfaceToggle />

      <main class="max-w-7xl mx-auto p-6 pt-20 lg:p-12 lg:pt-20">
        <div key={`${route}-${param}`} class="animate-page-enter">
          <RouteView route={route} param={param} />
        </div>
      </main>

      <MobileBar activeRoute={route} />
    </div>
  );
}

const AppLayout = () => {
  const queryClient = useQueryClient();

  const { data: authed, isLoading } = useQuery({
    queryKey: ["auth"],
    queryFn: () =>
      fetch("/api/auth/check", { credentials: "include" })
        .then((res) => res.ok)
        .catch(() => false),
    retry: false,
    staleTime: Infinity,
  });

  useEffect(() => {
    const onExpired = () => queryClient.setQueryData(["auth"], false);
    window.addEventListener("auth:expired", onExpired);
    return () => window.removeEventListener("auth:expired", onExpired);
  }, [queryClient]);

  if (isLoading) {
    return (
      <div class="min-h-screen flex items-center justify-center bg-bg">
        <div class="w-6 h-6 border-2 border-text/20 border-t-text/60 rounded-full animate-spin" />
      </div>
    );
  }

  if (!authed) {
    return <Login onSuccess={() => queryClient.setQueryData(["auth"], true)} />;
  }

  return (
    <SurfaceProvider>
      <AuthenticatedShell />
    </SurfaceProvider>
  );
};

export default AppLayout;
