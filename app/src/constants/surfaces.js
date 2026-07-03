import { RiBriefcase4Line, RiHomeHeartLine } from "@remixicon/react";

// The one place surface ids and their presentation live.
export const SURFACES = {
  life: {
    id: "life",
    label: "LifeOS",
    short: "Life",
    tagline: "Life Operating System",
    icon: RiHomeHeartLine,
    activeClass: "bg-accent text-white",
  },
  business: {
    id: "business",
    label: "BusinessOS",
    short: "Business",
    tagline: "Business Operating System",
    icon: RiBriefcase4Line,
    activeClass: "bg-status-info text-white",
  },
};

export const SURFACE_IDS = Object.keys(SURFACES);
