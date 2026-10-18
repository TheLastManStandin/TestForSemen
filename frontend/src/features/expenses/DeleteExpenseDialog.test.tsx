// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Expense } from "@/api/client";
import { TimeZoneContext } from "@/hooks/useToday";
import { DeleteExpenseDialog } from "./DeleteExpenseDialog";

const mutateAsync = vi.fn();
vi.mock("@/api/hooks", () => ({ useDeleteExpense: () => ({ mutateAsync, isPending: false }) }));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

function expense(id: number, comment: string): Expense {
  return {
    id, user_id: 1, member_name: "Папа", amount: "100.00", category: "food", spent_at: "2026-10-14",
    comment, created_at: "2026-10-14T10:00:00Z", updated_at: "2026-10-14T10:00:00Z", version: 1,
  };
}

const A = expense(1, "обед");
const B = expense(2, "такси");
const onClose = vi.fn();

function dialog(e: Expense | null) {
  return (
    <TimeZoneContext.Provider value="Europe/Moscow">
      <DeleteExpenseDialog expense={e} onClose={onClose} />
    </TimeZoneContext.Provider>
  );
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("DeleteExpenseDialog", () => {
  it("closes for the expense whose delete finished, even if another one is shown by then", async () => {
    const user = userEvent.setup();
    let finish!: () => void;
    mutateAsync.mockReturnValueOnce(new Promise<void>((r) => (finish = r)));

    const view = render(dialog(A));
    await user.click(screen.getByRole("button", { name: "Удалить" }));
    expect(mutateAsync).toHaveBeenCalledWith(A.id);

    view.rerender(dialog(B));
    await act(async () => finish());

    expect(onClose).toHaveBeenCalledWith(A);
    expect(onClose).not.toHaveBeenCalledWith(B);
  });
});
