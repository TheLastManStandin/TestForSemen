import { Plus } from "lucide-react";
import type { Member } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Filters } from "@/lib/filters";
import { CATEGORY_CODES, CATEGORY_LABELS, isCategory } from "@/lib/format";
import { PRESETS, type Preset } from "@/lib/period";

interface Props {
  filters: Filters;
  members: Member[];
  onChange: (patch: Partial<Filters>) => void;
  onCreate: () => void;
}

const ALL = "all";

export function FiltersBar({ filters, members, onChange, onCreate }: Props) {
  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="grid gap-1.5">
        <Label htmlFor="period">Период</Label>
        <Select value={filters.preset} onValueChange={(v) => onChange({ preset: v as Preset })}>
          <SelectTrigger id="period" className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PRESETS.map((p) => (
              <SelectItem key={p.value} value={p.value}>
                {p.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {filters.preset === "custom" && (
        <>
          <div className="grid gap-1.5">
            <Label htmlFor="from">С</Label>
            <Input id="from" type="date" className="w-40" value={filters.from} max={filters.to} onChange={(e) => e.target.value && onChange({ from: e.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="to">По</Label>
            <Input id="to" type="date" className="w-40" value={filters.to} min={filters.from} onChange={(e) => e.target.value && onChange({ to: e.target.value })} />
          </div>
        </>
      )}

      <div className="grid gap-1.5">
        <Label htmlFor="member">Член семьи</Label>
        <Select value={filters.memberId === null ? ALL : String(filters.memberId)} onValueChange={(v) => onChange({ memberId: v === ALL ? null : Number(v) })}>
          <SelectTrigger id="member" className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Все</SelectItem>
            {members.map((m) => (
              <SelectItem key={m.telegram_id} value={String(m.telegram_id)}>
                {m.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="filter-category">Категория</Label>
        <Select value={filters.category ?? ALL} onValueChange={(v) => onChange({ category: isCategory(v) ? v : null })}>
          <SelectTrigger id="filter-category" className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Все</SelectItem>
            {CATEGORY_CODES.map((c) => (
              <SelectItem key={c} value={c}>
                {CATEGORY_LABELS[c]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Button className="ml-auto" onClick={onCreate}>
        <Plus /> Добавить трату
      </Button>
    </div>
  );
}
