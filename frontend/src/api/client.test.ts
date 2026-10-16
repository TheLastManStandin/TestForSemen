import { describe, expect, it } from "vitest";
import { ApiError, conflictMessage, fieldErrors, isConflict, unwrap } from "./client";

function response(status: number): Response {
  return new Response(null, { status });
}

describe("unwrap", () => {
  it("returns data on success", async () => {
    await expect(unwrap(Promise.resolve({ data: { ok: true }, response: response(200) }))).resolves.toEqual({ ok: true });
  });

  it("throws ApiError with the problem message", async () => {
    const problem = { title: "Ошибка валидации", status: 422, errors: [{ field: "amount", message: "некорректная сумма" }] };
    const err = await unwrap(Promise.resolve({ error: problem, response: response(422) })).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(422);
    expect((err as ApiError).message).toBe("некорректная сумма");
  });

  it("falls back to the HTTP status", async () => {
    const err = await unwrap(Promise.resolve({ error: undefined, response: response(502) })).catch((e: unknown) => e);
    expect((err as ApiError).message).toBe("HTTP 502");
  });
});

describe("fieldErrors", () => {
  it("maps problem field errors", () => {
    const err = new ApiError(422, { title: "x", status: 422, errors: [{ field: "spent_at", message: "в будущем" }] });
    expect(fieldErrors(err)).toEqual({ spent_at: "в будущем" });
    expect(fieldErrors(new Error("boom"))).toEqual({});
  });
});

describe("isConflict", () => {
  it("detects a 409 problem", () => {
    expect(isConflict(new ApiError(409, { title: "Конфликт изменений", status: 409 }))).toBe(true);
    expect(isConflict(new ApiError(422))).toBe(false);
    expect(isConflict(new Error("x"))).toBe(false);
  });
});

describe("conflictMessage", () => {
  it("returns the server detail for a 409", () => {
    const err = new ApiError(409, { title: "Конфликт изменений", status: 409, detail: "Трата была изменена другим пользователем. Обновите данные и повторите." });
    expect(conflictMessage(err)).toBe("Трата была изменена другим пользователем. Обновите данные и повторите.");
  });

  it("falls back to a Russian text when the 409 has no detail", () => {
    expect(conflictMessage(new ApiError(409))).toMatch(/^Трата была изменена другим пользователем/);
  });

  it("is null for other errors", () => {
    expect(conflictMessage(new ApiError(422))).toBeNull();
    expect(conflictMessage(new Error("x"))).toBeNull();
  });
});
