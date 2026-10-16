import { describe, expect, it } from "vitest";
import { buildExpensePatch, type ExpenseFormValues } from "./expensePatch";

const base: ExpenseFormValues = { user_id: "111", amount: "350", category: "food", spent_at: "2026-10-14", comment: "кофе" };

describe("buildExpensePatch", () => {
  it("returns null when nothing changed", () => {
    expect(buildExpensePatch(base, { ...base }, 3)).toBeNull();
  });

  it("sends only the changed fields plus the loaded version", () => {
    expect(buildExpensePatch(base, { ...base, comment: "капучино" }, 3)).toEqual({ comment: "капучино", version: 3 });
    expect(buildExpensePatch(base, { ...base, amount: "1 200,50", category: "other" }, 1)).toEqual({
      amount: "1200.50",
      category: "other",
      version: 1,
    });
    expect(buildExpensePatch(base, { ...base, user_id: "222", spent_at: "2026-10-13" }, 2)).toEqual({
      user_id: 222,
      spent_at: "2026-10-13",
      version: 2,
    });
  });

  it("ignores formatting-only differences", () => {
    expect(buildExpensePatch(base, { ...base, amount: "350,00", comment: "  кофе " }, 1)).toBeNull();
    expect(buildExpensePatch({ ...base, amount: "350,5" }, { ...base, amount: "350.50" }, 1)).toBeNull();
  });

  it("trims the comment and allows clearing it", () => {
    expect(buildExpensePatch(base, { ...base, comment: "   " }, 4)).toEqual({ comment: "", version: 4 });
    expect(buildExpensePatch(base, { ...base, comment: " чай " }, 4)).toEqual({ comment: "чай", version: 4 });
  });
});
