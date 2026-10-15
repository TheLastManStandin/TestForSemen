import { useCallback, useEffect, useState } from "react";
import { applyPatch, parseFilters, serializeFilters, type Filters } from "@/lib/filters";
import { todayISO } from "@/lib/period";

/** Filter state mirrored in the URL query string. */
export function useFilters(): [Filters, (patch: Partial<Filters>) => void] {
  const [filters, setFilters] = useState(() => parseFilters(window.location.search, todayISO()));

  useEffect(() => {
    const onPop = () => setFilters(parseFilters(window.location.search, todayISO()));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const update = useCallback(
    (patch: Partial<Filters>) => {
      const next = applyPatch(filters, patch, todayISO());
      const qs = serializeFilters(next);
      window.history.replaceState(null, "", qs ? `?${qs}` : window.location.pathname);
      setFilters(next);
    },
    [filters],
  );

  return [filters, update];
}
