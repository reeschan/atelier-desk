import { useEffect, useMemo, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { Case, ReadWarning } from "../api/types";
import { errorText, t } from "../i18n";
import { causeName, ReadWarnings } from "./RunTable";

/** The order the classification values are known in; any other value goes after, by name. */
const KNOWN_ORDER = ["spec", "convention", "scope", "memory", "model", "env", "grader", "flaky"];

function orderOf(cause: string): number {
  const i = KNOWN_ORDER.indexOf(cause);
  return i < 0 ? KNOWN_ORDER.length : i;
}

/** The classifications that the data has, with the number of cases of each. */
export function causeCounts(cases: Case[]): { cause: string; n: number }[] {
  const counts = new Map<string, number>();
  for (const c of cases) {
    for (const cause of new Set(c.Causes ?? [])) {
      counts.set(cause, (counts.get(cause) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .map(([cause, n]) => ({ cause, n }))
    .sort((a, b) => orderOf(a.cause) - orderOf(b.cause) || (a.cause < b.cause ? -1 : 1));
}

/** The cases that have at least one of the checked causes; nothing checked keeps all. */
export function filterCases(cases: Case[], checked: ReadonlySet<string>): Case[] {
  if (checked.size === 0) return cases;
  return cases.filter((c) => (c.Causes ?? []).some((cause) => checked.has(cause)));
}

/**
 * The cases, narrowed by checking cause classifications. The classifications
 * are made from the values in the data. reloadKey changes when the set of
 * repositories changed.
 */
export function CaseTable({ reloadKey = 0 }: { reloadKey?: number }) {
  const client = useClient();
  const [list, setList] = useState<Case[] | null>(null);
  const [warnings, setWarnings] = useState<ReadWarning[]>([]);
  const [failed, setFailed] = useState("");
  const [checked, setChecked] = useState<ReadonlySet<string>>(new Set());

  useEffect(() => {
    let live = true;
    client
      .cases()
      .then((view) => {
        if (!live) return;
        const next = view.cases ?? [];
        // A cause that the reloaded list no longer has has no checkbox to uncheck: drop it.
        const present = new Set(causeCounts(next).map((c) => c.cause));
        setChecked((prev) => {
          const kept = [...prev].filter((cause) => present.has(cause));
          return kept.length === prev.size ? prev : new Set(kept);
        });
        setList(next);
        setWarnings(view.warnings ?? []);
        setFailed("");
      })
      .catch((e: unknown) => {
        if (live) setFailed(errorText(toDeskError(e).code));
      });
    return () => {
      live = false;
    };
  }, [client, reloadKey]);

  const all = useMemo(() => list ?? [], [list]);
  const counts = useMemo(() => causeCounts(all), [all]);
  const shown = useMemo(() => filterCases(all, checked), [all, checked]);
  const active = counts.filter((c) => checked.has(c.cause));

  function toggle(cause: string) {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(cause)) next.delete(cause);
      else next.add(cause);
      return next;
    });
  }

  return (
    <section aria-label={t("runs", "casesTitle")} className="flex flex-col gap-3">
      <h2 className="flex items-baseline justify-between text-sm">
        {t("runs", "casesTitle")}
        <span className="font-mono text-xs text-muted-foreground tabular-nums">
          {t("runs", "casesCount", { n: all.length })}
        </span>
      </h2>
      <ReadWarnings warnings={warnings} />
      {failed !== "" && (
        <p role="alert" className="text-xs text-destructive">
          {failed}
        </p>
      )}
      {counts.length > 0 && (
        <fieldset className="flex flex-wrap gap-1.5" aria-label={t("runs", "causeFilterLabel")}>
          {counts.map(({ cause, n }) => {
            const on = checked.has(cause);
            return (
              <label
                key={cause}
                className={`desk-tint inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-sm border px-2 text-xs ${
                  on
                    ? "border-primary/60 bg-accent text-accent-foreground"
                    : "border-foreground/10 text-foreground/75 hover:bg-card"
                }`}
              >
                <input
                  type="checkbox"
                  className="sr-only"
                  checked={on}
                  onChange={() => toggle(cause)}
                  aria-label={causeName(cause)}
                />
                <span aria-hidden="true">{causeName(cause)}</span>
                {causeName(cause) !== cause && (
                  <span className="font-mono text-muted-foreground" aria-hidden="true">
                    {cause}
                  </span>
                )}
                <span className="font-mono tabular-nums" aria-hidden="true">
                  {n}
                </span>
              </label>
            );
          })}
        </fieldset>
      )}
      {active.length > 0 && (
        <p className="text-xs text-muted-foreground tabular-nums">
          {t("runs", "casesFiltered", {
            causes: active.map((a) => causeName(a.cause)).join("・"),
            total: all.length,
            n: shown.length,
          })}
        </p>
      )}
      {list === null && failed === "" ? (
        <p className="text-sm text-muted-foreground">{t("runs", "loading")}</p>
      ) : all.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("runs", "casesEmpty")}</p>
      ) : shown.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("runs", "casesNoMatch")}</p>
      ) : (
        <ul className="divide-y divide-foreground/10 border-t border-foreground/10">
          {shown.map((c, i) => (
            <li
              key={`${c.Repo}|${c.WorkKey}|${c.PBI}|${i}`}
              data-archived={c.Archived ? "true" : "false"}
              className="flex flex-col gap-1 py-3"
            >
              <span className="flex items-center justify-between gap-3 font-mono text-[11px] text-muted-foreground">
                <span>
                  {c.Repo} · {c.WorkKey} · {c.PBI}
                  {c.Archived ? ` · ${t("runs", "archived")}` : ""}
                </span>
                <span>
                  {(c.Causes ?? []).length > 0
                    ? (c.Causes ?? []).map(causeName).join(" · ")
                    : t("runs", "causeNone")}
                </span>
              </span>
              <span className="text-sm leading-relaxed">{c.Summary}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
