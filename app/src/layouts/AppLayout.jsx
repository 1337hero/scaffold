import { inboxCountQuery } from "@/api/queries.js";
import CaptureModal from "@/components/CaptureModal.jsx";
import MobileBar from "@/components/MobileBar.jsx";
import Sidebar from "@/components/Sidebar.jsx";
import { useKeyboard } from "@/hooks/useKeyboard.js";
import { navigate, useRoute } from "@/hooks/useRoute.js";
import Coder from "@/pages/Coder.jsx";
import Dashboard from "@/pages/Dashboard.jsx";
import Inbox from "@/pages/Inbox.jsx";
import Login from "@/pages/Login.jsx";
import Area from "@/pages/Area.jsx";
import Areas from "@/pages/Areas.jsx";
import Search from "@/pages/Search.jsx";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "preact/hooks";

function RouteView({ route, param }) {
  switch (route) {
    case "dashboard":
      return <Dashboard />;
    case "inbox":
      return <Inbox />;
    case "areas":
      return param ? (
        <Area
          domainId={param}
          onBack={() => navigate("/areas")}
        />
      ) : (
        <Areas
          onOpenArea={(id) => navigate(`/areas/${id}`)}
        />
      );
    case "search":
      return <Search />;
    case "agents":
      return <Coder />;
    default:
      return <Dashboard />;
  }
}

function AuthenticatedShell() {
  const [captureOpen, setCaptureOpen] = useState(false);
  const { route, param } = useRoute();
  const queryClient = useQueryClient();
  const { data: inboxCount = 0 } = useQuery(inboxCountQuery);
  const { data: agentTasks = [] } = useQuery({
    queryKey: ["agent-tasks"],
    queryFn: () =>
      fetch("/api/agents/tasks", { credentials: "include" }).then((r) => r.json()),
    refetchInterval: 10_000,
  });
  const coderActive = (agentTasks ?? []).some((t) => t.status === "running");

  const openCapture = () => setCaptureOpen(true);
  const closeCapture = () => setCaptureOpen(false);

  const handleLogout = async () => {
    await fetch("/api/logout", { method: "POST", credentials: "include" });
    queryClient.invalidateQueries({ queryKey: ["auth"] });
  };

  useKeyboard([
    { key: "Escape", when: () => captureOpen, action: closeCapture },
    { key: "k", meta: true, action: () => (captureOpen ? closeCapture() : openCapture()) },
  ]);

  return (
    <div class="min-h-screen pb-24 lg:pb-0 lg:pl-64">
      <Sidebar
        activeRoute={route}
        onNavigate={navigate}
        onCapture={openCapture}
        onLogout={handleLogout}
        inboxCount={inboxCount}
        coderActive={coderActive}
      />

      <main class="max-w-7xl mx-auto p-6 lg:p-12">
        <div key={`${route}-${param}`} class="animate-page-enter">
          <RouteView route={route} param={param} />
        </div>
      </main>

      <MobileBar
        activeRoute={route}
        onNavigate={navigate}
        onCapture={openCapture}
      />

      <CaptureModal open={captureOpen} onClose={closeCapture} />
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
    return (
      <Login onSuccess={() => queryClient.setQueryData(["auth"], true)} />
    );
  }

  return <AuthenticatedShell />;
};

export default AppLayout;
