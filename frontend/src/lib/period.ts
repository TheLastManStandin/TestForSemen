export type Preset = "this_week" | "this_month" | "last_month" | "last_30_days" | "custom";

export const PRESETS: { value: Preset; label: string }[] = [
  { value: "this_week", label: "Эта неделя" },
  { value: "this_month", label: "Этот месяц" },
  { value: "last_month", label: "Прошлый месяц" },
  { value: "last_30_days", label: "30 дней" },
  { value: "custom", label: "Свой период" },
];

const DAY_MS = 86_400_000;

/** Local calendar date as "YYYY-MM-DD". */
export function toISODate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

export function todayISO(now: Date = new Date()): string {
  return toISODate(now);
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
