import { Bar, BarChart, CartesianGrid, LabelList, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { Member, Stats } from "@/api/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CATEGORY_LABELS, formatKopecks, formatShortDate, toKopecks } from "@/lib/format";
import { eachDay } from "@/lib/period";
import { memberColor } from "./memberColors";

interface Props {
  stats?: Stats;
  members: Member[];
  from: string;
  to: string;
}

const tick = { fill: "var(--muted-foreground)", fontSize: 12 };
const tooltipStyle = {
  background: "var(--popover)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  color: "var(--popover-foreground)",
};

function compactRub(rub: number): string {
  return rub >= 1000 ? `${Math.round(rub / 100) / 10}k` : String(rub);
}

export function Charts({ stats, members, from, to }: Props) {
  if (!stats || stats.count === 0) return null;

  const byCategory = stats.by_category.map((c) => ({ label: CATEGORY_LABELS[c.category], kopecks: toKopecks(c.total) }));

  const activeMembers = members.filter((m) => stats.by_member.some((b) => b.user_id === m.telegram_id));
  const totals = new Map(stats.by_day.map((d) => [`${d.date}|${d.user_id}`, toKopecks(d.total)]));
  const byDay = eachDay(from, to).map((day) => {
    const row: Record<string, string | number> = { day: formatShortDate(day) };
    for (const m of activeMembers) row[String(m.telegram_id)] = (totals.get(`${day}|${m.telegram_id}`) ?? 0) / 100;
    return row;
  });

  return (
    <div className="grid gap-4 lg:grid-cols-5">
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>По категориям</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={byCategory.length * 36 + 8}>
            <BarChart data={byCategory} layout="vertical" margin={{ left: 0, right: 96 }}>
              <XAxis type="number" hide />
              <YAxis type="category" dataKey="label" width={100} tick={tick} axisLine={false} tickLine={false} />
              <Tooltip cursor={{ fill: "var(--muted)" }} contentStyle={tooltipStyle} formatter={(v) => [formatKopecks(Number(v)), "Сумма"]} />
              <Bar dataKey="kopecks" fill="var(--chart-bar)" radius={[0, 4, 4, 0]} barSize={18}>
                <LabelList dataKey="kopecks" position="right" fill="var(--foreground)" fontSize={12} formatter={(v) => formatKopecks(Number(v))} />
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      <Card className="lg:col-span-3">
        <CardHeader>
          <CardTitle>По дням</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={byDay} margin={{ left: 0, right: 8 }}>
              <CartesianGrid vertical={false} stroke="var(--border)" />
              <XAxis dataKey="day" tick={tick} axisLine={false} tickLine={false} minTickGap={16} />
              <YAxis tick={tick} axisLine={false} tickLine={false} width={40} tickFormatter={(v) => compactRub(Number(v))} />
              <Tooltip
                cursor={{ fill: "var(--muted)" }}
                contentStyle={tooltipStyle}
                formatter={(v, name) => [formatKopecks(Number(v) * 100), members.find((m) => String(m.telegram_id) === name)?.name ?? String(name)]}
              />
              <Legend formatter={(value) => <span className="text-foreground">{members.find((m) => String(m.telegram_id) === value)?.name}</span>} />
              {activeMembers.map((m) => (
                <Bar key={m.telegram_id} dataKey={String(m.telegram_id)} stackId="day" fill={memberColor(members, m.telegram_id)} stroke="var(--card)" strokeWidth={2} />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>
    </div>
  );
}
