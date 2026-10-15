/** Splits server field errors into those the form can place and messages it cannot. */
export function splitFieldErrors(
  fields: Record<string, string>,
  known: readonly string[],
): { placed: [string, string][]; unplaced: string[] } {
  const placed: [string, string][] = [];
  const unplaced: string[] = [];
  for (const [field, message] of Object.entries(fields)) {
    if (known.includes(field)) placed.push([field, message]);
    else unplaced.push(message);
  }
  return { placed, unplaced };
}
