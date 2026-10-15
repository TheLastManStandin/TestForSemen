import { useState } from "react";
import { useExpenses } from "@/api/hooks";
import type { Expense, Member } from "@/api/client";
import type { Filters } from "@/lib/filters";
import { todayISO } from "@/lib/period";
import { DeleteExpenseDialog } from "./DeleteExpenseDialog";
import { ExpenseDialog } from "./ExpenseDialog";
import { ExpensesTable } from "./ExpensesTable";
import { FiltersBar } from "./FiltersBar";

interface Props {
  filters: Filters;
  onChange: (patch: Partial<Filters>) => void;
  members: Member[];
  highlighted: Set<number>;
}

type Editing = { mode: "create" } | { mode: "edit"; expense: Expense } | null;

export function ExpensesPage({ filters, onChange, members, highlighted }: Props) {
  const expenses = useExpenses(filters);
  const [editing, setEditing] = useState<Editing>(null);
  const [deleting, setDeleting] = useState<Expense | null>(null);

  return (
    <div className="grid gap-6">
      <FiltersBar filters={filters} members={members} onChange={onChange} onCreate={() => setEditing({ mode: "create" })} />
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
