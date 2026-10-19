// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type Expense, type Member } from "@/api/client";
import { TimeZoneContext } from "@/hooks/useToday";
import { ExpenseDialog } from "./ExpenseDialog";

const mutateAsync = vi.fn();
const refresh = vi.fn();

vi.mock("@/api/hooks", () => ({
  useCreateExpense: () => ({ mutateAsync }),
  useUpdateExpense: () => ({ mutateAsync }),
  useRefreshExpense: () => refresh,
}));
vi.mock("sonner", () => ({ toast: Object.assign(vi.fn(), { info: vi.fn(), error: vi.fn() }) }));

const members: Member[] = [{ telegram_id: 1, name: "Папа" }];

function expense(over: Partial<Expense>): Expense {
  return {
    id: 1, user_id: 1, member_name: "Папа", amount: "100.00", category: "food", spent_at: "2026-10-14",
    comment: "обед", created_at: "2026-10-14T10:00:00Z", updated_at: "2026-10-14T10:00:00Z", version: 1, ...over,
  };
}

const A = expense({ id: 1, amount: "100.00", comment: "обед" });
const B = expense({ id: 2, amount: "200.00", comment: "такси", category: "transport" });

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

function dialog(open: boolean, e: Expense | undefined) {
  return (
    <TimeZoneContext.Provider value="Europe/Moscow">
      <ExpenseDialog open={open} onOpenChange={onOpenChange} expense={e} members={members} defaultMemberId={null} />
    </TimeZoneContext.Provider>
  );
}

const onOpenChange = vi.fn();
const amount = () => screen.getByLabelText("Сумма, ₽") as HTMLInputElement;
const comment = () => screen.getByLabelText("Комментарий") as HTMLInputElement;

beforeEach(() => {
  // Radix primitives call these browser APIs that jsdom lacks.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.scrollIntoView ??= () => {};
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ExpenseDialog async results", () => {
  it("drops a conflict reload that finishes after another expense was opened", async () => {
    const user = userEvent.setup();
    mutateAsync.mockRejectedValueOnce(new ApiError(409, { title: "Conflict", status: 409, detail: "Трата изменена" }));
    const reload = deferred<Expense>();
    refresh.mockReturnValueOnce(reload.promise);

    const view = render(dialog(true, A));
    await user.clear(amount());
    await user.type(amount(), "150");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    await user.click(await screen.findByRole("button", { name: "Обновить данные" }));
    expect(refresh).toHaveBeenCalledWith(A.id);

    view.rerender(dialog(false, A));
    view.rerender(dialog(true, B));
    await act(async () => reload.resolve(expense({ id: 1, amount: "120.00", comment: "обед в кафе", version: 2 })));

    expect(amount().value).toBe("200");
    expect(comment().value).toBe("такси");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(mutateAsync).toHaveBeenCalledTimes(1);
    expect(mutateAsync.mock.calls[0][0].id).toBe(A.id);
  });

  it("keeps the reload button of the next expense usable while a stale reload is pending", async () => {
    const user = userEvent.setup();
    const conflict = new ApiError(409, { title: "Conflict", status: 409, detail: "Трата изменена" });
    mutateAsync.mockRejectedValueOnce(conflict).mockRejectedValueOnce(conflict);
    refresh.mockReturnValueOnce(new Promise(() => {}));

    const view = render(dialog(true, A));
    await user.clear(amount());
    await user.type(amount(), "150");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    await user.click(await screen.findByRole("button", { name: "Обновить данные" }));

    view.rerender(dialog(false, A));
    view.rerender(dialog(true, B));
    await user.clear(amount());
    await user.type(amount(), "250");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    const reload = (await screen.findByRole("button", { name: /^Обнов/ })) as HTMLButtonElement;
    expect(reload.textContent).toBe("Обновить данные");
    expect(reload.disabled).toBe(false);
  });

  it("does not close another expense when an earlier save finishes late", async () => {
    const user = userEvent.setup();
    const save = deferred<Expense>();
    mutateAsync.mockReturnValueOnce(save.promise);

    const view = render(dialog(true, A));
    await user.clear(amount());
    await user.type(amount(), "150");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(mutateAsync).toHaveBeenCalledTimes(1);

    view.rerender(dialog(false, A));
    onOpenChange.mockClear();
    view.rerender(dialog(true, B));
    await act(async () => save.resolve(expense({ id: 1, amount: "150.00", version: 2 })));

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(amount().value).toBe("200");
  });

  it("applies a conflict reload to the expense it was made for", async () => {
    const user = userEvent.setup();
    mutateAsync.mockRejectedValueOnce(new ApiError(409, { title: "Conflict", status: 409, detail: "Трата изменена" }));
    refresh.mockResolvedValueOnce(expense({ id: 1, amount: "120.00", comment: "обед в кафе", version: 2 }));

    render(dialog(true, A));
    await user.clear(amount());
    await user.type(amount(), "150");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));
    await user.click(await screen.findByRole("button", { name: "Обновить данные" }));

    expect(amount().value).toBe("150");
    expect(comment().value).toBe("обед в кафе");
  });
});
