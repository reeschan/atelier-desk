import { useEffect, useRef, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { Connection } from "../api/types";
import { errorText, t } from "../i18n";

interface Props {
  /** The version the screen shows now ("" when Atelier is not read). */
  version: string;
  /** True while the connection is being checked: the button does nothing. */
  disabled: boolean;
  /** The connection switchVersion answered with. */
  onSwitched: (c: Connection) => void;
  /**
   * Called just before switchVersion is asked. It returns the function that takes
   * the answer (and may drop it when a newer state already arrived by event);
   * without it, onSwitched takes the answer.
   */
  onSwitchStart?: () => (c: Connection) => void;
}

/** The version button and the list of versions under it. */
export function VersionPopover({ version, disabled, onSwitched, onSwitchStart }: Props) {
  const client = useClient();
  const root = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [versions, setVersions] = useState<string[] | null>(null);
  const [error, setError] = useState("");
  const [switching, setSwitching] = useState(false);

  // The list is read each time the popover opens, so a new versions/ entry shows.
  useEffect(() => {
    if (!open) return;
    let live = true;
    setVersions(null);
    client
      .versions()
      .then((vs) => {
        if (live) setVersions(vs);
      })
      .catch((e: unknown) => {
        if (live) {
          setVersions([]);
          setError(errorText(toDeskError(e).code));
        }
      });
    return () => {
      live = false;
    };
  }, [open, client]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  useEffect(() => {
    if (disabled) setOpen(false);
  }, [disabled]);

  function toggle() {
    setError("");
    setOpen((o) => !o);
  }

  async function choose(v: string) {
    if (switching || v === version) return;
    setSwitching(true);
    setError("");
    try {
      const apply = onSwitchStart ? onSwitchStart() : onSwitched;
      const c = await client.switchVersion(v);
      apply(c);
      setOpen(false);
    } catch (e) {
      setError(errorText(toDeskError(e).code));
    } finally {
      setSwitching(false);
    }
  }

  return (
    <div ref={root} className="relative">
      <button
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={t("common", "versionButtonLabel")}
        disabled={disabled}
        onClick={toggle}
        className="desk-tint inline-flex h-8 items-center gap-2 rounded-md border border-foreground/10 px-3 text-sm hover:bg-card disabled:cursor-not-allowed disabled:opacity-50"
      >
        <span className="text-muted-foreground">{t("common", "atelier")}</span>
        <span className="font-mono tabular-nums" data-testid="current-version">
          {version === "" ? t("common", "noVersion") : version}
        </span>
        <span className="text-xs text-muted-foreground">{t("common", "activeVersionLabel")}</span>
        <svg
          viewBox="0 0 24 24"
          width="16"
          height="16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="shrink-0 text-muted-foreground"
          aria-hidden="true"
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      {open && (
        <div className="desk-pop absolute top-10 left-0 z-20 w-72 rounded-lg border border-foreground/10 bg-card p-2 text-card-foreground shadow-md">
          <p className="px-2 pt-1 pb-2 text-xs text-muted-foreground">
            {t("common", "versionListTitle")}
          </p>
          {versions === null ? (
            <p className="px-2 py-1 text-xs text-muted-foreground">{t("common", "versionLoading")}</p>
          ) : versions.length === 0 ? (
            <p className="px-2 py-1 text-xs text-muted-foreground">{t("common", "versionListEmpty")}</p>
          ) : (
            <ul role="listbox" className="flex max-h-72 flex-col gap-0.5 overflow-y-auto">
              {versions.map((v) => (
                <li key={v} role="option" aria-selected={v === version}>
                  <button
                    type="button"
                    disabled={switching}
                    onClick={() => void choose(v)}
                    className={`desk-tint flex h-8 w-full items-center justify-between gap-6 rounded-sm px-2 text-left text-sm hover:bg-muted/60 ${
                      v === version ? "bg-muted" : ""
                    }`}
                  >
                    <span className="font-mono tabular-nums">{v}</span>
                    <span className="text-xs text-muted-foreground">
                      {v === version ? t("common", "versionCurrent") : ""}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          {error !== "" && (
            <p role="alert" className="mt-2 px-2 text-xs text-destructive">
              {error}
            </p>
          )}
          <p className="mt-2 border-t border-foreground/10 px-2 pt-2 text-xs text-muted-foreground">
            {switching ? t("common", "versionSwitching") : t("common", "versionSwitchNote")}
          </p>
        </div>
      )}
    </div>
  );
}
