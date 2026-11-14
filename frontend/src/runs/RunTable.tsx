import { useEffect, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { PBIResult, ReadWarning, Run } from "../api/types";
import { errorText, t } from "../i18n";
import { runs as runsTexts } from "../i18n/ja/runs";

/** The display name of a cause classification; a value nobody knows is shown as it is. */
export function causeName(cause: string): string {
  const names = runsTexts.causes as Record<string, string>;
  return Object.prototype.hasOwnProperty.call(names, cause) ? names[cause] : cause;
}

/** `2026-11-13T15:40:00+09:00` as `2026-11-13 15:40`, in the zone the record was written in. */
export function formatAt(at: unknown): string {
  if (typeof at !== "string" || at === "") return "";
  return at.replace("T", " ").slice(0, 16);
}

function atMillis(at: unknown): number {
  const ms = typeof at === "string" ? Date.parse(at) : Number.NaN;
  return Number.isNaN(ms) ? 0 : ms;
}

/** Runs by repository, then newest first. */
export function sortRuns(list: Run[]): Run[] {
  return [...list].sort((a, b) => {
    if (a.Repo !== b.Repo) return a.Repo < b.Repo ? -1 : 1;
    return atMillis(b.LastAt) - atMillis(a.LastAt);
  });
}

/** The count of lines that could not be read, with the lines behind a disclosure. */
export function ReadWarnings({ warnings }: { warnings: ReadWarning[] }) {
  if (warnings.length === 0) return null;
  return (
    <details className="text-xs text-warning">
      <summary className="cursor-pointer tabular-nums">
        {t("runs", "warningsSummary", { n: warnings.length })}
      </summary>
      <ul className="mt-1 flex flex-col gap-0.5 font-mono text-[11px] text-muted-foreground">
        {warnings.map((w, i) => (
          <li key={`${w.Path}:${w.Line}:${i}`}>
            {t("runs", "warningLine", { path: w.Path, line: w.Line, reason: w.Reason })}
          </li>
        ))}
      </ul>
    </details>
  );
}

function resultLabel(p: PBIResult): { text: string; dot: string } {
  if (p.Passed) return { text: t("runs", "resultPassed"), dot: "bg-success" };
  if (p.Held) return { text: t("runs", "resultHeld"), dot: "bg-warning" };
  const last = p.Attempts?.[p.Attempts.length - 1]?.Outcome;
  return {
    text: last !== undefined && last !== "" ? last : t("runs", "resultNotPassed"),
    dot: "bg-destructive",
  };
}

function lastCauses(p: PBIResult): string[] {
  const last = p.Attempts?.[p.Attempts.length - 1];
  return last?.Causes ?? [];
}

function runName(r: Run): string {
  return r.WorkKey !== "" ? r.WorkKey : r.Source !== "" ? r.Source : t("runs", "noWorkKey");
}

function runKey(r: Run, i: number): string {
  return `${r.Repo}|${r.WorkKey}|${r.Source}|${i}`;
}

function summary(r: Run) {
  const pbis = r.PBIs ?? [];
  return {
    total: pbis.length,
    passed: pbis.filter((p) => p.Passed).length,
    reworks: pbis.reduce((n, p) => n + p.Reworks, 0),
    escalations: pbis.reduce((n, p) => n + p.Escalations, 0),
    held: pbis.filter((p) => p.Held).length,
  };
}

function PbiResults({ run }: { run: Run }) {
  const pbis = run.PBIs ?? [];
  const s = summary(run);
  return (
    <section
      aria-label={t("runs", "resultsTitle")}
      className="flex min-w-0 flex-col gap-3 rounded-lg border border-foreground/10 bg-card p-5"
    >
      <h3 className="flex flex-wrap items-baseline gap-3">
        <span className="font-mono text-base">{runName(run)}</span>
        <span className="text-xs text-muted-foreground tabular-nums">
          {run.Repo} · {formatAt(run.LastAt)}
        </span>
      </h3>
      <dl className="grid grid-cols-4 gap-4 border-y border-foreground/10 py-3">
        <div>
          <dt className="text-xs text-muted-foreground">{t("runs", "statPassed")}</dt>
          <dd className="font-heading text-2xl tabular-nums">
            {s.passed}
            <span className="text-sm text-muted-foreground"> / {s.total}</span>
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">{t("runs", "statRework")}</dt>
          <dd className="font-heading text-2xl tabular-nums">{s.reworks}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">{t("runs", "statEscalation")}</dt>
          <dd className="font-heading text-2xl tabular-nums">{s.escalations}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">{t("runs", "statHeld")}</dt>
          <dd
            className={`font-heading text-2xl tabular-nums ${s.held > 0 ? "text-warning" : ""}`}
          >
            {s.held}
          </dd>
        </div>
      </dl>
      {pbis.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("runs", "resultsEmpty")}</p>
      ) : (
        <table className="w-full text-left">
          <thead>
            <tr className="text-xs text-muted-foreground">
              <th className="pb-2 font-normal">{t("runs", "colPbi")}</th>
              <th className="pb-2 font-normal">{t("runs", "colResult")}</th>
              <th className="pb-2 font-normal">{t("runs", "colModel")}</th>
              <th className="pb-2 font-normal">{t("runs", "colRework")}</th>
              <th className="pb-2 font-normal">{t("runs", "colEscalation")}</th>
            </tr>
          </thead>
          <tbody>
            {pbis.map((p) => {
              const result = resultLabel(p);
              const causes = lastCauses(p);
              return (
                <tr
                  key={p.ID}
                  className={`border-t border-foreground/10 ${p.Held ? "bg-warning/5" : ""}`}
                >
                  <td className="whitespace-nowrap py-2.5 pr-3 font-mono text-xs text-muted-foreground">
                    {p.ID}
                  </td>
                  <td className="py-2.5 pr-3 text-xs">
                    <span className="inline-flex items-center gap-1.5">
                      <span className={`size-1.5 rounded-full ${result.dot}`} aria-hidden="true" />
                      {result.text}
                    </span>
                    {p.Held && p.HoldReason !== "" && (
                      <span className="block font-mono text-[11px] text-muted-foreground">
                        {t("runs", "holdReason", { reason: causeName(p.HoldReason) })}
                      </span>
                    )}
                    {!p.Passed && causes.length > 0 && (
                      <span className="block font-mono text-[11px] text-muted-foreground">
                        {causes.map(causeName).join(" · ")}
                      </span>
                    )}
                  </td>
                  <td className="py-2.5 pr-3 font-mono text-xs">{p.Model}</td>
                  <td className="whitespace-nowrap py-2.5 pr-3 text-xs tabular-nums text-foreground/75">
                    {p.Reworks > 0 ? t("runs", "reworkCount", { n: p.Reworks }) : ""}
                  </td>
                  <td className="whitespace-nowrap py-2.5 font-mono text-xs tabular-nums text-foreground/75">
                    {p.Escalations > 0 ? t("runs", "escalationCount", { n: p.Escalations }) : ""}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </section>
  );
}

/**
 * The runs of the registered repositories by repository, newest first, with a
 * mark on the ones that came from the archive. A row opens the result of each
 * PBI of that run. reloadKey changes when the set of repositories changed.
 */
export function RunTable({ reloadKey = 0 }: { reloadKey?: number }) {
  const client = useClient();
  const [list, setList] = useState<Run[] | null>(null);
  const [warnings, setWarnings] = useState<ReadWarning[]>([]);
  const [failed, setFailed] = useState("");
  const [open, setOpen] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    client
      .runs()
      .then((view) => {
        if (!live) return;
        setList(sortRuns(view.runs ?? []));
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

  const rows = list ?? [];
  const opened = rows.findIndex((r, i) => runKey(r, i) === open);

  return (
    <section aria-label={t("runs", "runsTitle")} className="flex flex-col gap-3">
      <h2 className="flex items-baseline justify-between text-sm">
        {t("runs", "runsTitle")}
        <span className="font-mono text-xs text-muted-foreground tabular-nums">
          {t("runs", "runsCount", { n: rows.length })}
        </span>
      </h2>
      <ReadWarnings warnings={warnings} />
      {failed !== "" && (
        <p role="alert" className="text-xs text-destructive">
          {failed}
        </p>
      )}
      {list === null && failed === "" ? (
        <p className="text-sm text-muted-foreground">{t("runs", "loading")}</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("runs", "runsEmpty")}</p>
      ) : (
        <div className="grid grid-cols-[17rem_minmax(0,1fr)] gap-5">
          <ul className="flex flex-col gap-0.5">
            {rows.map((r, i) => {
              const key = runKey(r, i);
              const s = summary(r);
              const selected = key === open;
              return (
                <li key={key} data-repo={r.Repo} data-archived={r.Archived ? "true" : "false"}>
                  <button
                    type="button"
                    aria-expanded={selected}
                    onClick={() => setOpen(selected ? null : key)}
                    className={`desk-tint flex w-full flex-col gap-1 rounded-md px-3 py-2.5 text-left ${
                      selected ? "bg-card" : "hover:bg-card/60"
                    }`}
                  >
                    <span className="flex items-center justify-between font-mono text-xs text-muted-foreground tabular-nums">
                      <span>{formatAt(r.LastAt)}</span>
                      {r.Archived && <span>{t("runs", "archived")}</span>}
                    </span>
                    <span className="break-all font-mono text-sm">{runName(r)}</span>
                    <span className="flex justify-between gap-2 text-xs text-muted-foreground">
                      <span>
                        {r.Repo} · {t("runs", "pbiCount", { n: s.total })}
                      </span>
                      <span className={`tabular-nums ${s.held > 0 ? "text-warning" : ""}`}>
                        {[
                          t("runs", "summaryPassed", { n: s.passed }),
                          s.reworks > 0 ? t("runs", "summaryRework", { n: s.reworks }) : "",
                          s.held > 0 ? t("runs", "summaryHeld", { n: s.held }) : "",
                        ]
                          .filter((x) => x !== "")
                          .join(" · ")}
                      </span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
          {opened >= 0 ? (
            <PbiResults run={rows[opened]} />
          ) : (
            <p className="text-sm text-muted-foreground">{t("runs", "resultsHint")}</p>
          )}
        </div>
      )}
    </section>
  );
}
