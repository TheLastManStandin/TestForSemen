// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useToday } from "@/hooks/useToday";
import { SettingsGate } from "./SettingsGate";

const settings = vi.fn();
vi.mock("@/api/hooks", () => ({ useSettings: () => settings() }));

function Today() {
  const today = useToday();
  return <p>today {today()}</p>;
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("SettingsGate", () => {
  it("provides the family time zone once settings are loaded", () => {
    settings.mockReturnValue({ data: { timezone: "Europe/Moscow" }, isError: false });
    render(<SettingsGate><Today /></SettingsGate>);
    expect(screen.getByText(/^today \d{4}-\d{2}-\d{2}$/)).toBeTruthy();
  });

  it("shows an error instead of crashing on a time zone the browser does not know", () => {
    settings.mockReturnValue({ data: { timezone: "Local" }, isError: false });
    render(<SettingsGate><Today /></SettingsGate>);
    expect(screen.getByText(/Некорректный часовой пояс/)).toBeTruthy();
    expect(screen.queryByText(/^today/)).toBeNull();
  });

  it("offers a retry when settings fail to load", () => {
    settings.mockReturnValue({ data: undefined, isError: true, refetch: vi.fn() });
    render(<SettingsGate><Today /></SettingsGate>);
    expect(screen.getByRole("button", { name: "Повторить" })).toBeTruthy();
  });
});
