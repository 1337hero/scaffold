import {
  RiSunLine,
  RiTodoLine,
  RiFolder2Line,
  RiGroupLine,
  RiBookOpenLine,
  RiLayoutGridLine,
} from "@remixicon/react";

export const navItems = [
  { id: "today", path: "#/today", icon: RiSunLine, label: "Today" },
  { id: "tasks", path: "#/tasks", icon: RiTodoLine, label: "Tasks" },
  { id: "projects", path: "#/projects", icon: RiFolder2Line, label: "Projects" },
  { id: "people", path: "#/people", icon: RiGroupLine, label: "People" },
  { id: "library", path: "#/library", icon: RiBookOpenLine, label: "Library" },
  { id: "domains", path: "#/domains", icon: RiLayoutGridLine, label: "Domains" },
];
