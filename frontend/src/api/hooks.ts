import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Filters } from "@/lib/filters";
import { api, unwrap, type ExpenseCreate, type ExpensePatch } from "./client";

export const PAGE_SIZE = 20;
export const TRACES_PAGE_SIZE = 10;

export function useMembers() {
  return useQuery({
    queryKey: ["members"],
    queryFn: () => unwrap(api.GET("/members")),
    staleTime: 5 * 60_000,
  });
}

export function useExpenses(f: Filters) {
  return useQuery({
    queryKey: ["expenses", f.from, f.to, f.memberId, f.category, f.page],
    queryFn: () =>
      unwrap(
        api.GET("/expenses", {
          params: {
            query: {
              from: f.from,
              to: f.to,
              member_id: f.memberId ?? undefined,
              category: f.category ?? undefined,
              limit: PAGE_SIZE,
              offset: (f.page - 1) * PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  });
}

export function useStats(f: Filters) {
  return useQuery({
    queryKey: ["stats", f.from, f.to, f.memberId, f.category],
    queryFn: () =>
      unwrap(
        api.GET("/stats", {
          params: { query: { from: f.from, to: f.to, member_id: f.memberId ?? undefined, category: f.category ?? undefined } },
        }),
      ),
    placeholderData: keepPreviousData,
  });
}

export function useTraces(memberId: number | null, page: number) {
  return useQuery({
    queryKey: ["traces", memberId, page],
    queryFn: () =>
      unwrap(
        api.GET("/traces", {
          params: { query: { member_id: memberId ?? undefined, limit: TRACES_PAGE_SIZE, offset: (page - 1) * TRACES_PAGE_SIZE } },
        }),
      ),
    placeholderData: keepPreviousData,
    refetchInterval: 15_000,
  });
}

function useInvalidateExpenses() {
  const qc = useQueryClient();
  return () => Promise.all([qc.invalidateQueries({ queryKey: ["expenses"] }), qc.invalidateQueries({ queryKey: ["stats"] })]);
}

export function useCreateExpense() {
  const invalidate = useInvalidateExpenses();
  return useMutation({
    mutationFn: (body: ExpenseCreate) => unwrap(api.POST("/expenses", { body })),
    onSuccess: invalidate,
  });
}

export function useUpdateExpense() {
  const invalidate = useInvalidateExpenses();
  return useMutation({
    mutationFn: ({ id, patch }: { id: number; patch: ExpensePatch }) =>
      unwrap(api.PATCH("/expenses/{id}", { params: { path: { id } }, body: patch })),
    onSuccess: invalidate,
  });
}

/** Refetches one expense (after a 409) and refreshes the lists and stats around it. */
export function useRefreshExpense() {
  const invalidate = useInvalidateExpenses();
  return async (id: number) => {
    const [expense] = await Promise.all([unwrap(api.GET("/expenses/{id}", { params: { path: { id } } })), invalidate()]);
    return expense;
  };
}

export function useDeleteExpense() {
  const invalidate = useInvalidateExpenses();
  return useMutation({
    mutationFn: (id: number) => unwrap(api.DELETE("/expenses/{id}", { params: { path: { id } } })),
    onSuccess: invalidate,
  });
}
