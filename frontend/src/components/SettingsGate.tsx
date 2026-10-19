import type { ReactNode } from "react";
import { useSettings } from "@/api/hooks";
import { Button } from "@/components/ui/button";
import { TimeZoneContext } from "@/hooks/useToday";
import { isValidTimeZone } from "@/lib/period";

/** Renders the panel once the family settings are loaded: dates depend on the family's time zone. */
export function SettingsGate({ children }: { children: ReactNode }) {
  const settings = useSettings();
  if (settings.data) {
    const { timezone } = settings.data;
    if (!isValidTimeZone(timezone)) return <Failure message={`Некорректный часовой пояс семьи «${timezone}» — проверьте APP_TIMEZONE.`} />;
    return <TimeZoneContext.Provider value={timezone}>{children}</TimeZoneContext.Provider>;
  }
  if (settings.isError) return <Failure message="Не удалось загрузить настройки панели." onRetry={() => settings.refetch()} />;
  return <div className="min-h-screen bg-background" aria-busy />;
}

function Failure({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-background px-4 text-center text-muted-foreground">
      <p>{message}</p>
      {onRetry && (
        <Button variant="outline" onClick={onRetry}>
          Повторить
        </Button>
      )}
    </div>
  );
}
