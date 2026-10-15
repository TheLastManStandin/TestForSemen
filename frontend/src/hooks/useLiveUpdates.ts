import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import type { ExpenseEvent } from "@/api/client";

const EVENT_TYPES: ExpenseEvent["type"][] = ["expense.created", "expense.updated", "expense.deleted"];

/**
 * Subscribes to /api/events. EventSource reconnects by itself; on every (re)connect we
 * refetch everything, so changes made while offline are never missed.
 */
export function useLiveUpdates(onEvent: (e: ExpenseEvent) => void): boolean {
  const qc = useQueryClient();
  const [connected, setConnected] = useState(false);
  const handler = useRef(onEvent);

  useEffect(() => {
    handler.current = onEvent;
  }, [onEvent]);

  useEffect(() => {
    const source = new EventSource("/api/events");
    const onMessage = (msg: MessageEvent<string>) => {
      void qc.invalidateQueries({ queryKey: ["expenses"] });
      void qc.invalidateQueries({ queryKey: ["stats"] });
      void qc.invalidateQueries({ queryKey: ["traces"] });
      let event: ExpenseEvent;
      try {
        event = JSON.parse(msg.data) as ExpenseEvent;
      } catch (err) {
        console.warn("live updates: ignoring malformed event", err);
        return;
      }
      handler.current(event);
    };
    EVENT_TYPES.forEach((t) => source.addEventListener(t, onMessage));
    source.onopen = () => {
      setConnected(true);
      void qc.invalidateQueries();
    };
    source.onerror = () => setConnected(false);
    return () => source.close();
  }, [qc]);

  return connected;
}
