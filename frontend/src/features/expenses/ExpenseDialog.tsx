import { zodResolver } from "@hookform/resolvers/zod";
import { type FormEvent, useEffect, useRef, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { ApiError, conflictMessage, fieldErrors, type Expense, type Member } from "@/api/client";
import { useCreateExpense, useRefreshExpense, useUpdateExpense } from "@/api/hooks";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { buildExpensePatch } from "@/lib/expensePatch";
import { splitFieldErrors } from "@/lib/formErrors";
import { CATEGORY_CODES, CATEGORY_LABELS, normalizeAmount } from "@/lib/format";
import { todayISO } from "@/lib/period";

const schema = z.object({
  user_id: z.string().min(1, "Выберите, кто потратил"),
  amount: z.string().refine((v) => normalizeAmount(v) !== null, "Сумма, например 350 или 1 200,50"),
  category: z.enum(CATEGORY_CODES),
  spent_at: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "Укажите дату"),
  comment: z.string().max(500, "Не длиннее 500 символов"),
});

type FormValues = z.infer<typeof schema>;

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  expense?: Expense;
  members: Member[];
  defaultMemberId: number | null;
}

function initialValues(expense: Expense | undefined, defaultMemberId: number | null): FormValues {
  if (expense) {
    return {
      user_id: String(expense.user_id),
      amount: expense.amount.replace(/\.00$/, "").replace(".", ","),
      category: expense.category,
      spent_at: expense.spent_at,
      comment: expense.comment ?? "",
    };
  }
  return { user_id: defaultMemberId ? String(defaultMemberId) : "", amount: "", category: "food", spent_at: todayISO(), comment: "" };
}

export function ExpenseDialog({ open, onOpenChange, expense, members, defaultMemberId }: Props) {
  const create = useCreateExpense();
  const update = useUpdateExpense();
  const refreshExpense = useRefreshExpense();
  // The server state the edit is based on: the opened expense, or a fresh copy loaded after a conflict.
  const [reloaded, setReloaded] = useState<{ source: Expense; fresh: Expense } | null>(null);
  const base = expense && reloaded?.source === expense ? reloaded.fresh : expense;
  // A 409 is shown inside the dialog: a toast is not clickable while the modal dialog blocks the page.
  const [conflictState, setConflict] = useState<{ source: Expense; message: string } | null>(null);
  const conflict = expense && conflictState?.source === expense ? conflictState.message : null;
  const [reloading, setReloading] = useState(false);
  const form = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: initialValues(expense, defaultMemberId) });
  const { errors, isSubmitting } = form.formState;

  useEffect(() => {
    if (open) form.reset(initialValues(expense, defaultMemberId));
  }, [open, expense, defaultMemberId, form]);

  // Async handlers check this so a late response does not touch a dialog that is already closed.
  const openRef = useRef(open);
  useEffect(() => {
    openRef.current = open;
  }, [open]);

  const setOpen = (next: boolean) => {
    if (!next) {
      setReloaded(null);
      setConflict(null);
    }
    onOpenChange(next);
  };

  // Loads the current server version and keeps the user's edits on top of it, so they can review and save again.
  const reloadAfterConflict = async (source: Expense) => {
    setReloading(true);
    try {
      const fresh = await refreshExpense(source.id);
      if (!openRef.current) return;
      setReloaded({ source, fresh });
      setConflict(null);
      form.reset(initialValues(fresh, defaultMemberId), { keepDirtyValues: true });
      toast.info("Данные обновлены. Проверьте поля и сохраните ещё раз.");
    } catch (err) {
      if (!openRef.current) return;
      if (err instanceof ApiError && err.status === 404) {
        toast.error("Трату уже удалили.");
        setOpen(false);
      } else {
        setConflict({ source, message: `Не удалось обновить данные: ${err instanceof Error ? err.message : "ошибка сети"}` });
      }
    } finally {
      setReloading(false);
    }
  };

  // handleSubmit is invoked per event (not during render) because the handler reads openRef.
  const onSubmit = (e: FormEvent<HTMLFormElement>) =>
    form.handleSubmit(async (v) => {
      try {
        if (base) {
          const patch = buildExpensePatch(initialValues(base, defaultMemberId), v, base.version);
          if (patch) await update.mutateAsync({ id: base.id, patch });
        } else {
          await create.mutateAsync({
            user_id: Number(v.user_id),
            amount: normalizeAmount(v.amount)!,
            category: v.category,
            spent_at: v.spent_at,
            comment: v.comment.trim(),
          });
        }
        setOpen(false);
      } catch (err) {
        const message = conflictMessage(err);
        if (expense && message) {
          setConflict({ source: expense, message });
          return;
        }
        const { placed, unplaced } = splitFieldErrors(fieldErrors(err), Object.keys(schema.shape));
        for (const [field, message] of placed) form.setError(field as keyof FormValues, { message });
        if (unplaced.length > 0) toast.error(unplaced.join("; "));
        else if (placed.length === 0) toast.error(err instanceof Error ? err.message : "Не удалось сохранить");
      }
    })(e);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{expense ? "Изменить трату" : "Новая трата"}</DialogTitle>
          <DialogDescription>Изменения сразу увидят все члены семьи.</DialogDescription>
        </DialogHeader>
        {conflict && expense && (
          <div role="alert" className="grid gap-2 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
            <p>{conflict}</p>
            <p className="text-foreground/80">Ваши правки сохранятся в форме — после обновления проверьте поля и сохраните ещё раз.</p>
            <Button type="button" variant="outline" size="sm" className="justify-self-start" disabled={reloading} onClick={() => void reloadAfterConflict(expense)}>
              {reloading ? "Обновляем…" : "Обновить данные"}
            </Button>
          </div>
        )}
        <form id="expense-form" className="grid gap-4" onSubmit={onSubmit} noValidate>
          <div className="grid gap-1.5">
            <Label htmlFor="user_id">Кто потратил</Label>
            <Controller
              control={form.control}
              name="user_id"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="user_id" aria-invalid={!!errors.user_id}>
                    <SelectValue placeholder="Выберите" />
                  </SelectTrigger>
                  <SelectContent>
                    {members.map((m) => (
                      <SelectItem key={m.telegram_id} value={String(m.telegram_id)}>
                        {m.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            {errors.user_id && <p className="text-sm text-destructive">{errors.user_id.message}</p>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="amount">Сумма, ₽</Label>
              <Input id="amount" inputMode="decimal" autoComplete="off" placeholder="350" aria-invalid={!!errors.amount} {...form.register("amount")} />
              {errors.amount && <p className="text-sm text-destructive">{errors.amount.message}</p>}
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="spent_at">Дата</Label>
              <Input id="spent_at" type="date" aria-invalid={!!errors.spent_at} {...form.register("spent_at")} />
              {errors.spent_at && <p className="text-sm text-destructive">{errors.spent_at.message}</p>}
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="expense-category">Категория</Label>
            <Controller
              control={form.control}
              name="category"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="expense-category">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CATEGORY_CODES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {CATEGORY_LABELS[c]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="comment">Комментарий</Label>
            <Input id="comment" placeholder="кофе" aria-invalid={!!errors.comment} {...form.register("comment")} />
            {errors.comment && <p className="text-sm text-destructive">{errors.comment.message}</p>}
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            Отмена
          </Button>
          <Button type="submit" form="expense-form" disabled={isSubmitting}>
            {expense ? "Сохранить" : "Добавить"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
