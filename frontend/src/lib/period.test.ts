import { describe, expect, it } from "vitest";
import { addDays, daysInclusive, eachDay, isValidTimeZone, presetRange, todayISO } from "./period";

const today = "2026-10-15"; // Thursday

describe("presetRange", () => {
  it("resolves presets relative to today", () => {
    expect(presetRange("this_week", today)).toEqual({ from: "2026-10-12", to: today });
    expect(presetRange("this_month", today)).toEqual({ from: "2026-10-01", to: today });
    expect(presetRange("last_month", today)).toEqual({ from: "2026-09-01", to: "2026-09-30" });
    expect(presetRange("last_30_days", today)).toEqual({ from: "2026-09-16", to: today });
  });

  it("handles week and year edges", () => {
    expect(presetRange("this_week", "2026-10-18")).toEqual({ from: "2026-10-12", to: "2026-10-18" });
    expect(presetRange("this_week", "2026-10-12")).toEqual({ from: "2026-10-12", to: "2026-10-12" });
    expect(presetRange("last_month", "2026-01-10")).toEqual({ from: "2025-12-01", to: "2025-12-31" });
  });
});

describe("day helpers", () => {
  it("adds days across months", () => {
    expect(addDays("2026-10-01", -1)).toBe("2026-09-30");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });

  it("counts and lists days inclusively", () => {
    expect(daysInclusive("2026-10-01", "2026-10-15")).toBe(15);
    expect(eachDay("2026-09-29", "2026-10-02")).toEqual(["2026-09-29", "2026-09-30", "2026-10-01", "2026-10-02"]);
  });

  it("takes today in the family's time zone, not the browser's", () => {
    const lateEveningUTC = new Date("2026-10-18T21:30:00Z");
    expect(todayISO("Europe/Moscow", lateEveningUTC)).toBe("2026-10-19");
    expect(todayISO("America/New_York", lateEveningUTC)).toBe("2026-10-18");
    expect(todayISO("Asia/Tokyo", new Date("2026-12-31T15:00:00Z"))).toBe("2027-01-01");
  });

  it("tells IANA time zones from invalid ones", () => {
    expect(isValidTimeZone("Europe/Moscow")).toBe(true);
    expect(isValidTimeZone("Local")).toBe(false);
    expect(isValidTimeZone("")).toBe(false);
  });
});
