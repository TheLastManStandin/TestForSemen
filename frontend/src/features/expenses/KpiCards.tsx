import { TrendingDown, TrendingUp } from "lucide-react";
import type { Stats } from "@/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatKopecks, formatMoney, percentChange, toKopecks } from "@/lib/format";

interface Props {
  stats?: Stats;
  isLoading: boolean;
  days: number;
}

function Kpi({ title, value, hint, loading }: { title: string; value: React.ReactNode; hint?: React.ReactNode; loading: boolean }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-normal text-muted-foreground">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? <Skeleton className="h-8 w-32" /> : <div className="text-2xl font-semibold tabular-nums">{value}</div>}
        {hint && !loading && <div className="mt-1 text-xs text-muted-foreground">{hint}</div>}
      </CardContent>
    </Card>
  );
}

export function KpiCards({ stats, isLoading, days }: Props) {
  const total = stats ? toKopecks(stats.total) : 0;
  const prev = stats ? toKopecks(stats.prev_total) : 0;
  const change = percentChange(total, prev);
  const loading = isLoading || !stats;

  let changeValue: React.ReactNode = "—";
  if (change !== null) {
    const up = change > 0;
    const Icon = up ? TrendingUp : TrendingDown;
    changeValue = (
      <span className="inline-flex items-center gap-1.5">
        <Icon className="size-5" aria-hidden />
        {up ? "+" : ""}
        {change}%<span className="sr-only">{up ? " больше" : " меньше"}, чем в прошлом периоде</span>
      </span>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <Kpi title="Потрачено" value={stats && formatMoney(stats.total)} loading={loading} />
      <Kpi title="Трат" value={stats?.count} loading={loading} />
      <Kpi title="В среднем в день" value={formatKopecks(total / Math.max(days, 1))} hint={`за ${days} дн.`} loading={loading} />
      <Kpi title="К прошлому периоду" value={changeValue} hint={stats && `было ${formatMoney(stats.prev_total)}`} loading={loading} />
    </div>
  );
}
