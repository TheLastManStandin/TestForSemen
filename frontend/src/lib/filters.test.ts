import { describe, expect, it } from "vitest";
import { applyPatch, parseFilters, serializeFilters, type Filters } from "./filters";

const today = "2026-10-15";

const defaults: Filters = {
  preset: "this_month",
  from: "2026-10-01",
  to: today,
  memberId: null,
  category: null,
  page: 1,
  tab: "expenses",
  traceMemberId: null,
  tracePage: 1,
};

describe("parseFilters", () => {
  it("defaults to this month", () => {
    expect(parseFilters("", today)).toEqual(defaults);
  });

  it("reads all fields", () => {
    expect(parseFilters("?period=last_month&member=111&category=food&page=3&tab=traces", today)).toEqual({
      preset: "last_month",
      from: "2026-09-01",
      to: "2026-09-30",
      memberId: 111,
      category: "food",
      page: 3,
      tab: "traces",
      traceMemberId: null,
      tracePage: 1,
    });
  });

  it("reads a custom range", () => {
    const f = parseFilters("?period=custom&from=2026-08-01&to=2026-08-31", today);
    expect([f.preset, f.from, f.to]).toEqual(["custom", "2026-08-01", "2026-08-31"]);
  });

  it("falls back to defaults on garbage", () => {
    expect(parseFilters("?period=foo&member=abc&category=pets&page=-3&tab=admin", today)).toEqual(defaults);
    expect(parseFilters("?period=custom&from=2026-13-99&to=x", today)).toEqual(defaults);
    expect(parseFilters("?period=custom&from=2026-10-10&to=2026-10-01", today)).toEqual(defaults);
  });
});

describe("serializeFilters", () => {
  it("omits defaults", () => {
    expect(serializeFilters(defaults)).toBe("");
  });

  it("round-trips", () => {
    const f: Filters = { preset: "custom", from: "2026-08-01", to: "2026-08-31", memberId: 222, category: "health", page: 2, tab: "traces", traceMemberId: 111, tracePage: 4 };
    expect(parseFilters("?" + serializeFilters(f), today)).toEqual(f);
  });
});

describe("trace journal state", () => {
  it("reads member and page", () => {
    const f = parseFilters("?tab=traces&trace_member=111&trace_page=3", today);
    expect(f.traceMemberId).toBe(111);
    expect(f.tracePage).toBe(3);
  });

  it("falls back to defaults on garbage", () => {
    const f = parseFilters("?trace_member=abc&trace_page=-3", today);
    expect(f.traceMemberId).toBeNull();
    expect(f.tracePage).toBe(1);
  });

  it("serializes only non-default values and survives expense filter changes", () => {
    const f = applyPatch(defaults, { tab: "traces", traceMemberId: 111, tracePage: 2 }, today);
    expect(serializeFilters(f)).toBe("tab=traces&trace_member=111&trace_page=2");
    expect(applyPatch(f, { category: "food" }, today).tracePage).toBe(2);
  });

  it("resets the trace page when the member changes", () => {
    const f = { ...defaults, tracePage: 3 };
    expect(applyPatch(f, { traceMemberId: 222 }, today).tracePage).toBe(1);
    expect(applyPatch(f, { tracePage: 4 }, today).tracePage).toBe(4);
  });
});

describe("applyPatch", () => {
  it("recomputes the range for a preset", () => {
    const f = applyPatch(defaults, { preset: "this_week" }, today);
    expect([f.from, f.to]).toEqual(["2026-10-12", today]);
  });

  it("switches to custom when dates are edited", () => {
    const f = applyPatch(defaults, { from: "2026-09-20" }, today);
    expect(f.preset).toBe("custom");
    expect(f.from).toBe("2026-09-20");
  });

  it("resets the page when a filter changes but not when paging or switching tabs", () => {
    const onPage3 = { ...defaults, page: 3 };
    expect(applyPatch(onPage3, { memberId: 111 }, today).page).toBe(1);
    expect(applyPatch(onPage3, { tab: "traces" }, today).page).toBe(3);
    expect(applyPatch(onPage3, { page: 4 }, today).page).toBe(4);
  });
});
