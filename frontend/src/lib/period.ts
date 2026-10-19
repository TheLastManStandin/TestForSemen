export type Preset = "this_week" | "this_month" | "last_month" | "last_30_days" | "custom";

export const PRESETS: { value: Preset; label: string }[] = [
  { value: "this_week", label: "Эта неделя" },
  { value: "this_month", label: "Этот месяц" },
  { value: "last_month", label: "Прошлый месяц" },
  { value: "last_30_days", label: "30 дней" },
  { value: "custom", label: "Свой период" },
];

const DAY_MS = 86_400_000;

// Creating an Intl formatter is costly and "today" is read on every render.
const dayFormatters = new Map<string, Intl.DateTimeFormat>();

/** Today as "YYYY-MM-DD" in the family's time zone (Settings.timezone), as the bot counts it. Throws on an unknown zone. */
export function todayISO(timeZone: string, now: Date = new Date()): string {
  let f = dayFormatters.get(timeZone);
  if (!f) {
    f = new Intl.DateTimeFormat("en-US", { timeZone, year: "numeric", month: "2-digit", day: "2-digit" });
    dayFormatters.set(timeZone, f);
  }
  // Built from parts: the formatted string's layout depends on the browser's locale data.
  const parts = f.formatToParts(now);
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)!.value;
  return `${part("year")}-${part("month")}-${part("day")}`;
}

/** Whether the browser knows the IANA time zone. */
export function isValidTimeZone(timeZone: string): boolean {
  if (!timeZone) return false;
  try {
    todayISO(timeZone);
    return true;
  } catch {
    return false;
  }
}

function utc(iso: string): Date {
  return new Date(`${iso}T00:00:00Z`);
}

export function addDays(iso: string, n: number): string {
  return new Date(utc(iso).getTime() + n * DAY_MS).toISOString().slice(0, 10);
}

export function presetRange(p: Exclude<Preset, "custom">, todayIso: string): { from: string; to: string } {
  const firstOfMonth = `${todayIso.slice(0, 8)}01`;
  switch (p) {
    case "this_week": {
      const sinceMonday = (utc(todayIso).getUTCDay() + 6) % 7;
      return { from: addDays(todayIso, -sinceMonday), to: todayIso };
    }
    case "this_month":
      return { from: firstOfMonth, to: todayIso };
    case "last_month": {
      const lastOfPrev = addDays(firstOfMonth, -1);
      return { from: `${lastOfPrev.slice(0, 8)}01`, to: lastOfPrev };
    }
    case "last_30_days":
      return { from: addDays(todayIso, -29), to: todayIso };
  }
}

export function daysInclusive(from: string, to: string): number {
  return Math.round((utc(to).getTime() - utc(from).getTime()) / DAY_MS) + 1;
}

export function eachDay(from: string, to: string): string[] {
  const days: string[] = [];
  for (let d = from; d <= to; d = addDays(d, 1)) days.push(d);
  return days;
}
