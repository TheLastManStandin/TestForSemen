import { Radio, WifiOff } from "lucide-react";

export function LiveBadge({ connected }: { connected: boolean }) {
  return connected ? (
    <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground" title="Обновления приходят в реальном времени">
      <Radio className="size-4" aria-hidden /> Live
    </span>
  ) : (
    <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground" title="Переподключаемся…">
      <WifiOff className="size-4" aria-hidden /> Нет связи
    </span>
  );
}
