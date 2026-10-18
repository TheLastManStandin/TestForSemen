import { useCallback, useEffect, useState } from "react";
import { applyPatch, parseFilters, serializeFilters, type Filters } from "@/lib/filters";
import { useToday } from "./useToday";

/** Filter state mirrored in the URL query string. */
export function useFilters(): [Filters, (patch: Partial<Filters>) => void] {
  const today = useToday();
  const [filters, setFilters] = useState(() => parseFilters(window.location.search, today()));

  useEffect(() => {
    const onPop = () => setFilters(parseFilters(window.location.search, today()));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [today]);

  const update = useCallback(
    (patch: Partial<Filters>) => {
      const next = applyPatch(filters, patch, today());
      const qs = serializeFilters(next);
      window.history.replaceState(null, "", qs ? `?${qs}` : window.location.pathname);
      setFilters(next);
    },
    [filters, today],
  );

  return [filters, update];
}
