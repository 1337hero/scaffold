import { clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export const cn = (...inputs) => twMerge(clsx(inputs));

export function daysSince(dateStr) {
  if (!dateStr) return 0;
  const then = new Date(dateStr);
  if (Number.isNaN(then.getTime())) return 0;
  return Math.max(0, Math.floor((Date.now() - then.getTime()) / 86_400_000));
}
