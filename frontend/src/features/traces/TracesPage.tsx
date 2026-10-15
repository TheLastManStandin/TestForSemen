import { ChevronLeft, ChevronRight } from "lucide-react";
import type { Member } from "@/api/client";
import { TRACES_PAGE_SIZE, useTraces } from "@/api/hooks";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import type { Filters } from "@/lib/filters";
import { TraceCard } from "./TraceCard";

const ALL = "all";

interface Props {
  filters: Filters;
  onChange: (patch: Partial<Filters>) => void;
  members: Member[];
}

export function TracesPage({ filters, onChange, members }: Props) {
  const memberId = filters.traceMemberId;
  const page = filters.tracePage;
  const traces = useTraces(memberId, page);
  const total = traces.data?.total ?? 0;
  const lastPage = Math.max(1, Math.ceil(total / TRACES_PAGE_SIZE));

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="trace-member">Член семьи</Label>
          <Select
            value={memberId === null ? ALL : String(memberId)}
            onValueChange={(v) => onChange({ traceMemberId: v === ALL ? null : Number(v) })}
          >
            <SelectTrigger id="trace-member" className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Все</SelectItem>
              {members.map((m) => (
                <SelectItem key={m.telegram_id} value={String(m.telegram_id)}>
                  {m.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <p className="text-sm text-muted-foreground">Каждый запрос к боту: что понял агент, какие инструменты вызвал, сколько это стоило.</p>
      </div>

      {traces.isLoading && Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-28 w-full" />)}
      {!traces.isLoading && total === 0 && <p className="py-10 text-center text-muted-foreground">Агент ещё не получал сообщений.</p>}
      {traces.data?.items.map((t) => <TraceCard key={t.id} trace={t} />)}

      {total > TRACES_PAGE_SIZE && (
        <div className="flex items-center justify-end gap-2 text-sm text-muted-foreground">
          Страница {page} из {lastPage}
          <Button variant="outline" size="icon-sm" aria-label="Назад" disabled={page <= 1} onClick={() => onChange({ tracePage: page - 1 })}>
            <ChevronLeft />
          </Button>
          <Button variant="outline" size="icon-sm" aria-label="Вперёд" disabled={page >= lastPage} onClick={() => onChange({ tracePage: page + 1 })}>
            <ChevronRight />
          </Button>
        </div>
      )}
    </div>
  );
}
