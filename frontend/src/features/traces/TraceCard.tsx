import { ChevronDown, ChevronRight, Keyboard, Mic } from "lucide-react";
import { useState } from "react";
import type { Trace } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { formatDateTime } from "@/lib/format";

function pretty(v: unknown): string {
  return JSON.stringify(v, null, 2);
}

export function TraceCard({ trace }: { trace: Trace }) {
  const [open, setOpen] = useState(false);
  const voice = trace.source === "voice";

  return (
    <Card>
      <CardContent className="grid gap-3">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
          <span className="font-medium">{trace.member_name}</span>
          <Badge variant="outline" className="gap-1">
            {voice ? <Mic className="size-3" /> : <Keyboard className="size-3" />}
            {voice ? "голос" : "текст"}
          </Badge>
          <span className="text-muted-foreground">{formatDateTime(trace.created_at)}</span>
          <span className="ml-auto text-xs text-muted-foreground tabular-nums">
            {(trace.latency_ms / 1000).toFixed(1)} с
            {trace.stt_ms !== null && ` (распознавание ${(trace.stt_ms / 1000).toFixed(1)} с)`} · {trace.prompt_tokens + trace.completion_tokens} ток. · {trace.model}
          </span>
        </div>

        <div className="grid gap-1 text-sm">
          <p>
            <span className="text-muted-foreground">→ </span>
            {trace.input}
          </p>
          {trace.output && (
            <p className="whitespace-pre-line">
              <span className="text-muted-foreground">← </span>
              {trace.output}
            </p>
          )}
          {trace.error && <p className="text-destructive">Ошибка: {trace.error}</p>}
        </div>

        {trace.steps.length > 0 && (
          <div>
            <Button variant="ghost" size="sm" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
              {open ? <ChevronDown /> : <ChevronRight />} Вызовы инструментов ({trace.steps.length})
            </Button>
            {open && (
              <ol className="mt-2 grid gap-2">
                {trace.steps.map((s, i) => (
                  <li key={i} className="rounded-lg border bg-muted/40 p-3 text-xs">
                    <div className="mb-2 flex justify-between font-mono">
                      <span className="font-semibold">{s.tool}</span>
                      <span className="text-muted-foreground">{s.ms} мс</span>
                    </div>
                    <div className="grid gap-2 md:grid-cols-2">
                      <pre className="max-h-48 overflow-auto rounded bg-background p-2">{pretty(s.args)}</pre>
                      <pre className="max-h-48 overflow-auto rounded bg-background p-2">{pretty(s.result)}</pre>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
