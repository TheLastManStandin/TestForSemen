import type { Member } from "@/api/client";

const SLOTS = 4;

/** Categorical color for a member: fixed by position in the member list, never by rank; no cycling. */
export function memberColor(members: Member[], userId: number): string {
  const i = members.findIndex((m) => m.telegram_id === userId);
  return i >= 0 && i < SLOTS ? `var(--series-${i + 1})` : "var(--muted-foreground)";
}
