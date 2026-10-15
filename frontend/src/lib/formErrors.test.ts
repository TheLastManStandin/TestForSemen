import { describe, expect, it } from "vitest";
import { splitFieldErrors } from "./formErrors";

describe("splitFieldErrors", () => {
  it("separates known fields from unplaceable ones", () => {
    const r = splitFieldErrors({ amount: "bad", date: "future", body: "oops" }, ["amount", "spent_at"]);
    expect(r.placed).toEqual([["amount", "bad"]]);
    expect(r.unplaced).toEqual(["future", "oops"]);
  });

  it("returns empty lists for no errors", () => {
    expect(splitFieldErrors({}, ["amount"])).toEqual({ placed: [], unplaced: [] });
  });
});
