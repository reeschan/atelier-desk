import { useCallback, useEffect, useRef, useState } from "react";
import { useClient } from "../api/client";
import { watchConnection } from "../api/events";
import type { ConnState, Connection } from "../api/types";
import { t } from "../i18n";

/**
 * The connection to Atelier, kept current: the first connection() answer and
 * every connection:changed after it. Null until the first answer. `set` takes
 * the answer of switchVersion / setPythonPath, which is the newest state too.
 */
export function useConnection(): [
  Connection | null,
  (c: Connection) => void,
  () => (c: Connection) => void,
] {
  const client = useClient();
  const [conn, setConn] = useState<Connection | null>(null);
  // Counts what watchConnection delivered, so an answer can tell whether a newer state arrived first.
  const seq = useRef(0);
  useEffect(
    () =>
      watchConnection(client, (c) => {
        seq.current += 1;
        setConn(c);
      }),
    [client],
  );
  // Call before asking (switchVersion); the returned function applies the answer
  // only when no connection:changed arrived since, because that event is newer.
  const begin = useCallback(() => {
    const at = seq.current;
    return (c: Connection) => {
      if (seq.current === at) setConn(c);
    };
  }, []);
  return [conn, setConn, begin];
}

const LOOK: Record<ConnState, { box: string; dot: string; label: string }> = {
  checking: {
    box: "bg-muted text-muted-foreground",
    dot: "border border-muted-foreground",
    label: "connChecking",
  },
  ready: { box: "bg-success/10 text-foreground", dot: "bg-success", label: "connReady" },
  python_unavailable: {
    box: "bg-destructive/15 text-foreground",
    dot: "bg-destructive",
    label: "connPythonUnavailable",
  },
  unsupported: {
    box: "bg-warning/15 text-foreground",
    dot: "bg-warning",
    label: "connUnsupported",
  },
};

/** The state of the connection as a coloured badge. It scales in once when it appears (and again when the state changes). */
export function ConnectionBadge({ state }: { state: ConnState }) {
  const look = LOOK[state] ?? LOOK.checking;
  return (
    <span
      key={state}
      role="status"
      aria-label={t("common", "connLabel")}
      data-conn-state={state}
      className={`desk-badge-in inline-flex h-7 items-center gap-2 rounded-md px-2.5 text-xs ${look.box}`}
    >
      <span className={`size-2 rounded-full ${look.dot}`} aria-hidden="true" />
      {t("common", look.label)}
    </span>
  );
}
