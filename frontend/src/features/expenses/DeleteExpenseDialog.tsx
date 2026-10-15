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
import { CATEGORY_LABELS, formatDate, formatMoney } from "@/lib/format";
import { todayISO } from "@/lib/period";

interface Props {
  expense: Expense | null;
  onClose: () => void;
}

export function DeleteExpenseDialog({ expense, onClose }: Props) {
  const remove = useDeleteExpense();

  const confirm = async () => {
    if (!expense) return;
    try {
      await remove.mutateAsync(expense.id);
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Не удалось удалить");
    }
  };

  return (
    <AlertDialog open={expense !== null} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Удалить трату?</AlertDialogTitle>
          <AlertDialogDescription>
            {expense &&
              `${expense.member_name}: «${expense.comment || CATEGORY_LABELS[expense.category]}» на ${formatMoney(expense.amount)}, ${formatDate(expense.spent_at, todayISO())}. Это действие нельзя отменить.`}
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
