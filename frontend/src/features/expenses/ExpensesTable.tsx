import { ChevronLeft, ChevronRight, Pencil, Trash2 } from "lucide-react";
import { PAGE_SIZE } from "@/api/hooks";
import type { Expense, ExpenseList, Member } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CATEGORY_LABELS, formatDate, formatMoney } from "@/lib/format";
import { memberColor } from "./memberColors";

interface Props {
  data?: ExpenseList;
  isLoading: boolean;
  isError: boolean;
  members: Member[];
  page: number;
  onPage: (page: number) => void;
  onEdit: (e: Expense) => void;
  onDelete: (e: Expense) => void;
  highlighted: Set<number>;
  todayIso: string;
}

export function ExpensesTable({ data, isLoading, isError, members, page, onPage, onEdit, onDelete, highlighted, todayIso }: Props) {
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const first = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const last = Math.min(page * PAGE_SIZE, total);
  const showError = isError && !isLoading && items.length === 0;

  return (
    <div className="rounded-xl border bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-28">Дата</TableHead>
            <TableHead>Кто</TableHead>
            <TableHead>Категория</TableHead>
            <TableHead>Комментарий</TableHead>
            <TableHead className="text-right">Сумма</TableHead>
            <TableHead className="w-24" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading &&
            Array.from({ length: 5 }, (_, i) => (
              <TableRow key={i}>
                <TableCell colSpan={6}>
                  <Skeleton className="h-6 w-full" />
                </TableCell>
              </TableRow>
            ))}
          {showError && (
            <TableRow>
              <TableCell colSpan={6} role="alert" className="py-10 text-center text-destructive">
                Не удалось загрузить траты
              </TableCell>
            </TableRow>
          )}
          {!isLoading && !isError && items.length === 0 && (
            <TableRow>
              <TableCell colSpan={6} className="py-10 text-center text-muted-foreground">
                Трат за этот период нет. Напишите боту «кофе 350» или добавьте трату здесь.
              </TableCell>
            </TableRow>
          )}
          {items.map((e) => (
            <TableRow key={e.id} className={highlighted.has(e.id) ? "animate-highlight" : undefined}>
              <TableCell className="whitespace-nowrap">{formatDate(e.spent_at, todayIso)}</TableCell>
              <TableCell>
                <span className="inline-flex items-center gap-2">
                  <span className="size-2.5 rounded-full" style={{ background: memberColor(members, e.user_id) }} aria-hidden />
                  {e.member_name}
                </span>
              </TableCell>
              <TableCell>
                <Badge variant="outline">{CATEGORY_LABELS[e.category]}</Badge>
              </TableCell>
              <TableCell className="max-w-64 truncate text-muted-foreground">{e.comment ?? "—"}</TableCell>
              <TableCell className="text-right font-medium tabular-nums">{formatMoney(e.amount)}</TableCell>
              <TableCell className="text-right">
                <Button variant="ghost" size="icon-sm" aria-label="Изменить" onClick={() => onEdit(e)}>
                  <Pencil />
                </Button>
                <Button variant="ghost" size="icon-sm" aria-label="Удалить" onClick={() => onDelete(e)}>
                  <Trash2 />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <div className="flex items-center justify-between gap-2 border-t px-4 py-3 text-sm text-muted-foreground">
        <span>
          {total > 0 ? `${first}–${last} из ${total}` : "0 трат"}
          {data && total > 0 && <> · всего {formatMoney(data.sum)}</>}
        </span>
        <div className="flex gap-1">
          <Button variant="outline" size="icon-sm" aria-label="Назад" disabled={page <= 1} onClick={() => onPage(page - 1)}>
            <ChevronLeft />
          </Button>
          <Button variant="outline" size="icon-sm" aria-label="Вперёд" disabled={last >= total} onClick={() => onPage(page + 1)}>
            <ChevronRight />
          </Button>
        </div>
      </div>
    </div>
  );
}
