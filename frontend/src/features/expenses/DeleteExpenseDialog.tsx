import { toast } from "sonner";
import type { Expense } from "@/api/client";
import { useDeleteExpense } from "@/api/hooks";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useToday } from "@/hooks/useToday";
import { CATEGORY_LABELS, formatDate, formatMoney } from "@/lib/format";

interface Props {
  expense: Expense | null;
  /** Called with the expense the dialog was closed for: a late delete may finish after another one is shown. */
  onClose: (expense: Expense) => void;
}

export function DeleteExpenseDialog({ expense, onClose }: Props) {
  const remove = useDeleteExpense();
  const today = useToday();

  const confirm = async () => {
    if (!expense) return;
    const target = expense;
    try {
      await remove.mutateAsync(target.id);
      onClose(target);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Не удалось удалить");
    }
  };

  return (
    <AlertDialog open={expense !== null} onOpenChange={(open) => !open && expense && onClose(expense)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Удалить трату?</AlertDialogTitle>
          <AlertDialogDescription>
            {expense &&
              `${expense.member_name}: «${expense.comment || CATEGORY_LABELS[expense.category]}» на ${formatMoney(expense.amount)}, ${formatDate(expense.spent_at, today())}. Это действие нельзя отменить.`}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Отмена</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(e) => {
              e.preventDefault();
              void confirm();
            }}
          >
            Удалить
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
