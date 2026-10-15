import { Wallet } from "lucide-react";
import { useMembers } from "@/api/hooks";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Toaster } from "@/components/ui/sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ExpensesPage } from "@/features/expenses/ExpensesPage";
import { useFilters } from "@/hooks/useFilters";
import type { Tab } from "@/lib/filters";

const NO_HIGHLIGHTS = new Set<number>();

export default function App() {
  const [filters, update] = useFilters();
  const members = useMembers().data ?? [];

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-2">
            <Wallet className="size-5" />
            <h1 className="text-lg font-semibold">Семейные расходы</h1>
          </div>
          <ThemeToggle />
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Tabs value={filters.tab} onValueChange={(v) => update({ tab: v as Tab })}>
          <TabsList className="mb-4">
            <TabsTrigger value="expenses">Траты</TabsTrigger>
            <TabsTrigger value="traces">Журнал агента</TabsTrigger>
          </TabsList>
          <TabsContent value="expenses">
            <ExpensesPage filters={filters} onChange={update} members={members} highlighted={NO_HIGHLIGHTS} />
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
