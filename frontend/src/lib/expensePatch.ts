import type { Category, ExpensePatch } from "@/api/client";
import { normalizeAmount, toKopecks } from "./format";

/** Raw values of the expense form. */
export interface ExpenseFormValues {
  user_id: string;
  amount: string;
  category: Category;
  spent_at: string;
  comment: string;
}

function sameAmount(a: string, b: string): boolean {
  const na = normalizeAmount(a);
  const nb = normalizeAmount(b);
  return na !== null && nb !== null ? toKopecks(na) === toKopecks(nb) : a === b;
}

/**
 * PATCH body with only the fields the user changed relative to the loaded expense, plus the
 * version it was loaded at (so a concurrent change is reported as 409 instead of being
 * overwritten). Returns null if nothing changed. `current` must already pass form validation.
 */
export function buildExpensePatch(initial: ExpenseFormValues, current: ExpenseFormValues, version: number): ExpensePatch | null {
  const patch: ExpensePatch = {};
  if (current.user_id !== initial.user_id) patch.user_id = Number(current.user_id);
  if (!sameAmount(current.amount, initial.amount)) patch.amount = normalizeAmount(current.amount)!;
  if (current.category !== initial.category) patch.category = current.category;
  if (current.spent_at !== initial.spent_at) patch.spent_at = current.spent_at;
  const comment = current.comment.trim();
  if (comment !== initial.comment.trim()) patch.comment = comment;
  return Object.keys(patch).length === 0 ? null : { ...patch, version };
}
