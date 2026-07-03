import { todayQuery } from "@/api/queries.js";
import CalendarSection from "@/components/today/CalendarSection.jsx";
import NotificationsSection from "@/components/today/NotificationsSection.jsx";
import SlippingSection from "@/components/today/SlippingSection.jsx";
import Top3Section from "@/components/today/Top3Section.jsx";
import { useSurface } from "@/hooks/useSurface.jsx";
import { useQuery } from "@tanstack/react-query";

const Today = () => {
  const { surface } = useSurface();
  const { data, isLoading, error } = useQuery(todayQuery(surface));

  if (isLoading) {
    return <p class="text-app-muted">Loading today…</p>;
  }
  if (error) {
    return <p class="text-status-error">Couldn't load Today: {error.message}</p>;
  }

  return (
    <div class="space-y-6">
      <h1 class="font-serif italic text-3xl font-semibold tracking-tight">Today</h1>

      <Top3Section tasks={data.top3} surface={surface} />

      <div class="grid gap-6 lg:grid-cols-2">
        <CalendarSection events={data.calendar} />
        <SlippingSection slipping={data.slipping} />
      </div>

      <NotificationsSection notifications={data.notifications} />
    </div>
  );
};

export default Today;
