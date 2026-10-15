import { describe, expect, it } from "vitest";
import {
  formatDate,
  formatKopecks,
  formatMoney,
  formatShortDate,
  isCategory,
  normalizeAmount,
  percentChange,
  toKopecks,
} from "./format";

const NB = " ";

describe("money", () => {
  it("converts API decimals to kopecks without floats", () => {
    expect(toKopecks("350.00")).toBe(35000);
    expect(toKopecks("1200.5")).toBe(120050);
    expect(toKopecks("0.07")).toBe(7);
    expect(toKopecks("19.99")).toBe(1999);
  });

  it("formats kopecks for humans", () => {
    expect(formatKopecks(35000)).toBe(`350${NB}₽`);
    expect(formatKopecks(120050)).toBe(`1${NB}200,50${NB}₽`);
    expect(formatKopecks(123456789)).toBe(`1${NB}234${NB}567,89${NB}₽`);
    expect(formatKopecks(0)).toBe(`0${NB}₽`);
    expect(formatMoney("1200.50")).toBe(`1${NB}200,50${NB}₽`);
  });

  it("normalizes user input for the API", () => {
    expect(normalizeAmount("350")).toBe("350");
    expect(normalizeAmount(" 1 200,50 ")).toBe("1200.50");
    expect(normalizeAmount("1200.5")).toBe("1200.5");
    expect(normalizeAmount("0")).toBeNull();
    expect(normalizeAmount("abc")).toBeNull();
    expect(normalizeAmount("1,234,5")).toBeNull();
    expect(normalizeAmount("12.345")).toBeNull();
  });
});

describe("dates", () => {
  it("omits the current year", () => {
    expect(formatDate("2026-10-14", "2026-10-15")).toBe("14 окт");
    expect(formatDate("2025-05-01", "2026-10-15")).toBe("1 мая 2025");
    expect(formatShortDate("2026-10-04")).toBe("04.10");
  });
});

describe("misc", () => {
  it("computes percent change", () => {
    expect(percentChange(110, 100)).toBe(10);
    expect(percentChange(50, 100)).toBe(-50);
    expect(percentChange(10, 0)).toBeNull();
  });

  it("recognizes categories", () => {
    expect(isCategory("food")).toBe(true);
    expect(isCategory("pets")).toBe(false);
    expect(isCategory(null)).toBe(false);
  });
});
