import { lazy, Suspense, useState } from "react";
import { useExpenses, useStats } from "@/api/hooks";
import type { Expense, Member } from "@/api/client";
import { Skeleton } from "@/components/ui/skeleton";
import type { Filters } from "@/lib/filters";
import { daysInclusive, todayISO } from "@/lib/period";
import { DeleteExpenseDialog } from "./DeleteExpenseDialog";
import { ExpenseDialog } from "./ExpenseDialog";
import { ExpensesTable } from "./ExpensesTable";
import { FiltersBar } from "./FiltersBar";
import { KpiCards } from "./KpiCards";

interface Props {
  filters: Filters;
  onChange: (patch: Partial<Filters>) => void;
  members: Member[];
  highlighted: Set<number>;
}

// Recharts is the heaviest dependency; load it in its own chunk.
const Charts = lazy(() => import("./Charts").then((m) => ({ default: m.Charts })));

type Editing = { mode: "create" } | { mode: "edit"; expense: Expense } | null;

export function ExpensesPage({ filters, onChange, members, highlighted }: Props) {
  const expenses = useExpenses(filters);
  const stats = useStats(filters);
  const [editing, setEditing] = useState<Editing>(null);
  const [deleting, setDeleting] = useState<Expense | null>(null);

  return (
    <div className="grid gap-6">
      <FiltersBar filters={filters} members={members} onChange={onChange} onCreate={() => setEditing({ mode: "create" })} />
      <KpiCards stats={stats.data} isLoading={stats.isLoading} days={daysInclusive(filters.from, filters.to)} />
      <Suspense fallback={<Skeleton className="h-72 w-full" />}>
        <Charts stats={stats.data} members={members} from={filters.from} to={filters.to} />
      </Suspense>
      <ExpensesTable
        data={expenses.data}
        isLoading={expenses.isLoading}
        isError={expenses.isError}
        members={members}
        page={filters.page}
        onPage={(page) => onChange({ page })}
        onEdit={(expense) => setEditing({ mode: "edit", expense })}
        onDelete={setDeleting}
        highlighted={highlighted}
        todayIso={todayISO()}
      />
      <ExpenseDialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
        expense={editing?.mode === "edit" ? editing.expense : undefined}
        members={members}
        defaultMemberId={filters.memberId}
      />
      <DeleteExpenseDialog expense={deleting} onClose={() => setDeleting(null)} />
    </div>
  );
}
