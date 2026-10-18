import { createContext, useCallback, useContext } from "react";
import { todayISO } from "@/lib/period";

/** The family's time zone from /settings; provided by SettingsGate. */
export const TimeZoneContext = createContext<string | null>(null);

/** Returns a function giving today as "YYYY-MM-DD" in the family's time zone, as the bot counts it. */
export function useToday(): () => string {
  const timeZone = useContext(TimeZoneContext);
  if (!timeZone) throw new Error("useToday is used outside SettingsGate");
  return useCallback(() => todayISO(timeZone), [timeZone]);
}
