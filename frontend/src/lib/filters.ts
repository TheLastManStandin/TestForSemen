import { isCategory, type CategoryCode } from "./format";
import { presetRange, type Preset } from "./period";

export type Tab = "expenses" | "traces";

export interface Filters {
  preset: Preset;
  from: string;
  to: string;
  memberId: number | null;
  category: CategoryCode | null;
  page: number;
  tab: Tab;
}

const PRESET_VALUES: Preset[] = ["this_week", "this_month", "last_month", "last_30_days", "custom"];
const DEFAULT_PRESET = "this_month";
const ISO_DATE = /^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$/;

function defaults(todayIso: string): Filters {
  return { preset: DEFAULT_PRESET, ...presetRange(DEFAULT_PRESET, todayIso), memberId: null, category: null, page: 1, tab: "expenses" };
}

/** Parses the URL query; anything invalid falls back to its default. */
export function parseFilters(search: string, todayIso: string): Filters {
  const p = new URLSearchParams(search);
  const f = defaults(todayIso);

  const preset = p.get("period");
  if (preset && (PRESET_VALUES as string[]).includes(preset)) {
    if (preset === "custom") {
      const from = p.get("from") ?? "";
      const to = p.get("to") ?? "";
      if (ISO_DATE.test(from) && ISO_DATE.test(to) && from <= to) {
        Object.assign(f, { preset: "custom", from, to });
      }
    } else {
      Object.assign(f, { preset, ...presetRange(preset as Exclude<Preset, "custom">, todayIso) });
    }
  }

  const member = Number(p.get("member"));
  if (Number.isSafeInteger(member) && member > 0) f.memberId = member;

  const category = p.get("category");
  if (isCategory(category)) f.category = category;

  const page = Number(p.get("page"));
  if (Number.isInteger(page) && page > 1) f.page = page;

  if (p.get("tab") === "traces") f.tab = "traces";
  return f;
}

/** Serializes only non-default values, so the default view has a clean URL. */
export function serializeFilters(f: Filters): string {
  const p = new URLSearchParams();
  if (f.preset !== DEFAULT_PRESET) p.set("period", f.preset);
  if (f.preset === "custom") {
    p.set("from", f.from);
    p.set("to", f.to);
  }
  if (f.memberId !== null) p.set("member", String(f.memberId));
  if (f.category !== null) p.set("category", f.category);
  if (f.page > 1) p.set("page", String(f.page));
  if (f.tab !== "expenses") p.set("tab", f.tab);
  return p.toString();
}

/** Applies a UI change: presets recompute dates, edited dates mean "custom", filter changes reset the page. */
export function applyPatch(f: Filters, patch: Partial<Filters>, todayIso: string): Filters {
  const next = { ...f, ...patch };
  if (patch.preset && patch.preset !== "custom") {
    Object.assign(next, presetRange(patch.preset, todayIso));
  }
  if ((patch.from !== undefined || patch.to !== undefined) && patch.preset === undefined) {
    next.preset = "custom";
  }
  const filterKeys: (keyof Filters)[] = ["preset", "from", "to", "memberId", "category"];
  if (filterKeys.some((k) => k in patch) && !("page" in patch)) {
    next.page = 1;
  }
  return next;
}
