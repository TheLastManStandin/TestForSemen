import { describe, expect, it } from "vitest";
import { ApiError, fieldErrors, unwrap } from "./client";

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
