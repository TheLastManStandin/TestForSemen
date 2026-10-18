import type { ReactNode } from "react";
import { useSettings } from "@/api/hooks";
import { Button } from "@/components/ui/button";
import { TimeZoneContext } from "@/hooks/useToday";

/** Renders the panel once the family settings are loaded: dates depend on the family's time zone. */
export function SettingsGate({ children }: { children: ReactNode }) {
  const settings = useSettings();
  if (settings.data) return <TimeZoneContext.Provider value={settings.data.timezone}>{children}</TimeZoneContext.Provider>;
  if (settings.isError) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-background text-muted-foreground">
        <p>Не удалось загрузить настройки панели.</p>
        <Button variant="outline" onClick={() => settings.refetch()}>
          Повторить
        </Button>
      </div>
    );
  }
  return <div className="min-h-screen bg-background" aria-busy />;
}
