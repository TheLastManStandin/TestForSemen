import { Wallet } from "lucide-react";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import type { ExpenseEvent } from "@/api/client";
import { useMembers } from "@/api/hooks";
import { LiveBadge } from "@/components/LiveBadge";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Toaster } from "@/components/ui/sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ExpensesPage } from "@/features/expenses/ExpensesPage";
import { useFilters } from "@/hooks/useFilters";
import { useLiveUpdates } from "@/hooks/useLiveUpdates";
import type { Tab } from "@/lib/filters";
import { CATEGORY_LABELS, formatMoney } from "@/lib/format";

const EVENT_VERB: Record<ExpenseEvent["type"], string> = {
  "expense.created": "Новая трата",
  "expense.updated": "Трата изменена",
  "expense.deleted": "Трата удалена",
};

const HIGHLIGHT_MS = 3000;

export default function App() {
  const [filters, update] = useFilters();
  const members = useMembers().data ?? [];
  const [highlighted, setHighlighted] = useState<Set<number>>(() => new Set());

  const onEvent = useCallback((ev: ExpenseEvent) => {
    const e = ev.expense;
    toast(`${e.member_name}: ${e.comment ?? CATEGORY_LABELS[e.category]} — ${formatMoney(e.amount)}`, { description: EVENT_VERB[ev.type] });
    if (ev.type === "expense.deleted") return;
    setHighlighted((prev) => new Set(prev).add(e.id));
    setTimeout(() => {
      setHighlighted((prev) => {
        const next = new Set(prev);
        next.delete(e.id);
        return next;
      });
    }, HIGHLIGHT_MS);
  }, []);
  const connected = useLiveUpdates(onEvent);

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-2">
            <Wallet className="size-5" />
            <h1 className="text-lg font-semibold">Семейные расходы</h1>
          </div>
          <div className="flex items-center gap-3">
            <LiveBadge connected={connected} />
            <ThemeToggle />
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Tabs value={filters.tab} onValueChange={(v) => update({ tab: v as Tab })}>
          <TabsList className="mb-4">
            <TabsTrigger value="expenses">Траты</TabsTrigger>
            <TabsTrigger value="traces">Журнал агента</TabsTrigger>
          </TabsList>
          <TabsContent value="expenses">
            <ExpensesPage filters={filters} onChange={update} members={members} highlighted={highlighted} />
          </TabsContent>
          <TabsContent value="traces">
            <p className="text-muted-foreground">Скоро здесь появится журнал агента.</p>
          </TabsContent>
        </Tabs>
      </main>
      <Toaster position="bottom-right" />
    </div>
  );
}
