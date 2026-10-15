import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

export const api = createClient<paths>({ baseUrl: "/api" });

type Schemas = components["schemas"];
export type Category = Schemas["Category"];
export type Expense = Schemas["Expense"];
export type ExpenseCreate = Schemas["ExpenseCreate"];
export type ExpensePatch = Schemas["ExpensePatch"];
export type ExpenseList = Schemas["ExpenseList"];
export type ExpenseEvent = Schemas["ExpenseEvent"];
export type Member = Schemas["Member"];
export type Stats = Schemas["Stats"];
export type Trace = Schemas["Trace"];
export type TraceStep = Schemas["TraceStep"];
export type Problem = Schemas["Problem"];

export class ApiError extends Error {
  readonly status: number;
  readonly problem?: Problem;

  constructor(status: number, problem?: Problem) {
    super(problem?.errors?.[0]?.message ?? problem?.detail ?? problem?.title ?? `HTTP ${status}`);
    this.status = status;
    this.problem = problem;
  }
}

/** Resolves an openapi-fetch call to its data or throws ApiError. */
export async function unwrap<T>(call: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await call;
  if (error !== undefined || !response.ok) {
    throw new ApiError(response.status, error as Problem | undefined);
  }
  return data as T;
}

/** Field -> message map from a 422 problem (empty for other errors). */
export function fieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError) || !err.problem?.errors) return {};
  return Object.fromEntries(err.problem.errors.map((e) => [e.field, e.message]));
}
