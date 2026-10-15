export const CATEGORY_CODES = ["food", "transport", "housing", "entertainment", "health", "shopping", "other"] as const;

export type CategoryCode = (typeof CATEGORY_CODES)[number];

export const CATEGORY_LABELS: Record<CategoryCode, string> = {
  food: "Еда",
  transport: "Транспорт",
  housing: "Жильё",
  entertainment: "Развлечения",
  health: "Здоровье",
  shopping: "Покупки",
  other: "Другое",
};

export function isCategory(v: unknown): v is CategoryCode {
  return typeof v === "string" && (CATEGORY_CODES as readonly string[]).includes(v);
}

const NBSP = " ";

/** "1200.50" -> 120050. API amounts are decimal strings; never parse them as floats. */
export function toKopecks(amount: string): number {
  const [whole, frac = ""] = amount.trim().split(".");
  return Number(whole) * 100 + Number((frac + "00").slice(0, 2));
}

/** 120050 -> "1 200,50 ₽" (no-break spaces). */
export function formatKopecks(kopecks: number): string {
  const abs = Math.abs(Math.round(kopecks));
  const whole = Math.floor(abs / 100)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, NBSP);
  const kop = abs % 100;
  const sign = kopecks < 0 ? "−" : "";
  return `${sign}${whole}${kop ? "," + String(kop).padStart(2, "0") : ""}${NBSP}₽`;
}

export function formatMoney(amount: string): string {
  return formatKopecks(toKopecks(amount));
}

/** User input "1 200,50" -> "1200.50" for the API; null if not a positive amount. */
export function normalizeAmount(input: string): string | null {
  const clean = input.replace(/\s/g, "").replace(",", ".");
  if (!/^\d{1,13}(\.\d{1,2})?$/.test(clean)) return null;
  return Number(clean) > 0 ? clean : null;
}

const MONTHS = ["янв", "фев", "мар", "апр", "мая", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"];

/** "2026-10-14" -> "14 окт" (or "14 окт 2025" for another year). */
export function formatDate(iso: string, todayIso: string): string {
  const [y, m, d] = iso.split("-").map(Number);
  const base = `${d} ${MONTHS[m - 1]}`;
  return iso.slice(0, 4) === todayIso.slice(0, 4) ? base : `${base} ${y}`;
}

/** "2026-10-04" -> "04.10" for chart axes. */
export function formatShortDate(iso: string): string {
  return `${iso.slice(8, 10)}.${iso.slice(5, 7)}`;
}

/** RFC 3339 timestamp -> "15 окт, 12:30" in the browser's time zone. */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${d.getDate()} ${MONTHS[d.getMonth()]}, ${hh}:${mm}`;
}

export function percentChange(current: number, previous: number): number | null {
  if (previous === 0) return null;
  return Math.round(((current - previous) / previous) * 100);
}
