import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import Markdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { toDeskError, useClient } from "../api/client";
import { watchConnection } from "../api/events";
import type { SkillView } from "../api/types";
import { errorText, t } from "../i18n";
import { navigate } from "../shell/routes";

const DISABLE_KEY = "disable-model-invocation";

const MD_COMPONENTS: Components = {
  h1: ({ children }) => <h3 className="mt-2 font-heading text-xl">{children}</h3>,
  h2: ({ children }) => <h4 className="mt-2 text-base font-bold">{children}</h4>,
  h3: ({ children }) => <h5 className="mt-1 text-sm font-bold">{children}</h5>,
  p: ({ children }) => <p className="text-foreground/75">{children}</p>,
  ul: ({ children }) => <ul className="flex list-disc flex-col gap-1.5 pl-5 text-foreground/75">{children}</ul>,
  ol: ({ children }) => <ol className="flex list-decimal flex-col gap-1.5 pl-5 text-foreground/75">{children}</ol>,
  pre: ({ children }) => (
    <pre className="overflow-x-auto rounded-md border border-foreground/10 bg-background p-4 font-mono text-xs leading-relaxed text-foreground/75">
      {children}
    </pre>
  ),
  code: ({ children, className }) => (
    <code className={className ?? "rounded-sm bg-muted px-1 font-mono text-xs"}>{children}</code>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-xs">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border border-foreground/10 bg-muted px-2 py-1 font-bold">{children}</th>
  ),
  td: ({ children }) => <td className="border border-foreground/10 px-2 py-1">{children}</td>,
};

function formatValue(v: unknown): string {
  if (typeof v === "string") return v;
  if (v === null || v === undefined) return "";
  if (typeof v === "object") return JSON.stringify(v, null, 2);
  return String(v);
}

type DetailState =
  | { status: "loading" }
  | { status: "error"; code: string }
  | { status: "ok"; view: SkillView };

type FileState =
  | { rel: string; status: "loading" }
  | { rel: string; status: "ok"; text: string }
  | { rel: string; status: "error"; code: string };

function Section({ label, title, note, children }: { label: string; title: string; note?: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-foreground/10 bg-card p-5" aria-label={label}>
      <h2 className="text-sm">
        {title}
        {note !== undefined && <span className="ml-2 text-xs text-muted-foreground">{note}</span>}
      </h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}

export function SkillDetail({ skillKey }: { skillKey: string }) {
  const client = useClient();
  const [state, setState] = useState<DetailState>({ status: "loading" });
  const [file, setFile] = useState<FileState | null>(null);

  // The last connection state and version seen. The used-by section depends on the catalog of the
  // connection, so it is shown only for a ready connection and read again when that changes.
  const seen = useRef<{ state: string; version: string } | undefined>(undefined);
  const [connState, setConnState] = useState<string | undefined>(undefined);
  // True while the used-by of a previous version could be on screen: a read for the new version is pending.
  const [usedByStale, setUsedByStale] = useState(false);
  // Counts reads; a read whose number is no longer current (a newer read started, the skill changed
  // or the screen unmounted) drops its answer.
  const seq = useRef(0);

  useEffect(() => {
    let live = true;
    const read = () => {
      const mine = ++seq.current;
      client
        .skill(skillKey)
        .then((view) => {
          if (!live || mine !== seq.current) return;
          setState({ status: "ok", view });
          setUsedByStale(false);
        })
        .catch((e: unknown) => {
          if (!live || mine !== seq.current) return;
          setState({ status: "error", code: toDeskError(e).code });
          setUsedByStale(false);
        });
    };
    setState({ status: "loading" });
    setFile(null);
    setUsedByStale(false);
    read();
    const offHome = client.events.onHomeChanged(() => {
      if (live && seen.current?.state === "ready") read();
    });
    const offConnection = watchConnection(client, (c) => {
      if (!live) return;
      const prev = seen.current;
      seen.current = { state: c.state, version: c.version };
      setConnState(c.state);
      const versionChanged = prev === undefined || prev.version !== c.version;
      const becameReady = c.state === "ready" && prev?.state !== "ready";
      if (!versionChanged && !becameReady) return;
      if (prev !== undefined && versionChanged) setUsedByStale(true);
      read();
    });
    return () => {
      live = false;
      seq.current++;
      offHome();
      offConnection();
    };
  }, [client, skillKey]);

  const openFile = (rel: string) => {
    setFile({ rel, status: "loading" });
    client
      .skillFile(skillKey, rel)
      .then((text) => setFile((cur) => (cur?.rel === rel ? { rel, status: "ok", text } : cur)))
      .catch((e: unknown) =>
        setFile((cur) => (cur?.rel === rel ? { rel, status: "error", code: toDeskError(e).code } : cur)),
      );
  };

  const view = state.status === "ok" ? state.view : null;
  const changelog = useMemo(() => {
    const entries = view?.detail.Versions?.Changelog ?? [];
    // newest first: "Unreleased" (no time) on top, then by time, later lines first on a tie
    return [...entries].reverse().sort((a, b) => {
      if (a.At === b.At) return 0;
      if (a.At === "") return -1;
      if (b.At === "") return 1;
      return a.At < b.At ? 1 : -1;
    });
  }, [view]);

  if (state.status === "loading") {
    return (
      <p className="p-8 text-sm text-muted-foreground" role="status">
        {t("skills", "detailLoading")}
      </p>
    );
  }
  if (state.status === "error") {
    return (
      <div className="p-8 text-sm" role="alert">
        <p>{t("skills", "detailFailed")}</p>
        <p className="mt-1 text-muted-foreground">{errorText(state.code)}</p>
      </div>
    );
  }

  const { detail, usedBy } = state.view;
  const sk = detail.Skill;
  const fm = detail.Frontmatter ?? {};
  const fmKeys = Object.keys(fm);
  const hasDisableKey = fmKeys.includes(DISABLE_KEY);
  const autoRow = (
    <tr key={DISABLE_KEY}>
      <th scope="row" className="w-48 py-1.5 pr-4 text-left align-top font-normal text-muted-foreground">
        {t("skills", "autoInvokeLabel")}
        <span className="block font-mono text-[11px]">{DISABLE_KEY}</span>
      </th>
      <td className="py-1.5">
        {sk.DisableModelInvocation ? t("skills", "autoInvokeNo") : t("skills", "autoInvokeYes")}
      </td>
    </tr>
  );
  const versions = detail.Versions;
  const files = detail.Files ?? [];
  const used = usedBy ?? [];

  return (
    <div className="flex flex-col gap-6 px-8 py-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-heading text-3xl">{sk.Name}</h1>
        <p className="font-mono text-xs text-muted-foreground">{sk.Key}</p>
        <p className="text-sm text-foreground/75">
          {sk.Description === "" ? t("skills", "noDescription") : sk.Description}
        </p>
      </header>

      <Section label={t("skills", "frontmatterTitle")} title={t("skills", "frontmatterTitle")}>
        <table className="w-full text-sm">
          <thead className="sr-only">
            <tr>
              <th scope="col">{t("skills", "fmKey")}</th>
              <th scope="col">{t("skills", "fmValue")}</th>
            </tr>
          </thead>
          <tbody>
            {fmKeys.map((k) =>
              k === DISABLE_KEY ? (
                autoRow
              ) : (
                <tr key={k}>
                  <th scope="row" className="w-48 py-1.5 pr-4 text-left align-top font-mono text-xs font-normal text-muted-foreground">
                    {k}
                  </th>
                  <td className="whitespace-pre-wrap py-1.5 leading-relaxed">{formatValue(fm[k])}</td>
                </tr>
              ),
            )}
            {!hasDisableKey && autoRow}
          </tbody>
        </table>
      </Section>

      <article
        className="flex flex-col gap-3 text-sm leading-relaxed"
        aria-label={t("skills", "bodyTitle")}
      >
        <div className="border-b border-foreground/10 pb-2">
          <h2 className="font-mono text-xs text-muted-foreground">{t("skills", "bodyTitle")}</h2>
        </div>
        {detail.Body.trim() === "" ? (
          <p className="text-muted-foreground">{t("skills", "bodyEmpty")}</p>
        ) : (
          <Markdown remarkPlugins={[remarkGfm]} components={MD_COMPONENTS}>
            {detail.Body}
          </Markdown>
        )}
      </article>

      <Section
        label={t("skills", "filesTitle")}
        title={t("skills", "filesTitle")}
        note={t("skills", "filesCount", { count: files.length })}
      >
        {files.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("skills", "filesEmpty")}</p>
        ) : (
          <ul className="flex flex-col gap-0.5">
            {files.map((f) => (
              <li key={f.RelPath}>
                <button
                  type="button"
                  onClick={() => openFile(f.RelPath)}
                  aria-pressed={file?.rel === f.RelPath}
                  className={
                    "desk-tint flex h-8 w-full items-center gap-2 rounded-sm px-2 text-left " +
                    (file?.rel === f.RelPath ? "bg-accent text-accent-foreground" : "hover:bg-muted/60")
                  }
                >
                  <span className="font-mono text-xs">{f.RelPath}</span>
                  <span className="ml-auto font-mono text-xs text-muted-foreground">
                    {t("skills", "fileSize", { size: f.Size })}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        {file !== null && (
          <div className="mt-3 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="font-mono text-xs text-muted-foreground">{file.rel}</span>
              <button
                type="button"
                onClick={() => setFile(null)}
                className="desk-tint text-xs text-muted-foreground hover:text-foreground"
              >
                {t("skills", "fileClose")}
              </button>
            </div>
            {file.status === "loading" && (
              <p className="text-xs text-muted-foreground" role="status">
                {t("skills", "fileLoading")}
              </p>
            )}
            {file.status === "error" && (
              <p className="text-xs text-destructive" role="alert">
                {file.code === "too_large"
                  ? t("skills", "fileTooLarge")
                  : file.code === "binary"
                    ? t("skills", "fileBinary")
                    : `${t("skills", "fileFailed")} (${errorText(file.code)})`}
              </p>
            )}
            {file.status === "ok" && (
              <pre className="max-h-96 overflow-auto rounded-md border border-foreground/10 bg-background p-4 font-mono text-xs leading-relaxed text-foreground/75">
                {file.text}
              </pre>
            )}
          </div>
        )}
      </Section>

      <Section label={t("skills", "versionsTitle")} title={t("skills", "versionsTitle")}>
        {versions === undefined || versions === null ? (
          <p className="text-sm text-muted-foreground">{t("skills", "versionsNone")}</p>
        ) : (
          <div className="flex flex-col gap-4">
            <ul className="flex flex-wrap gap-2" aria-label={t("skills", "versionsListLabel")}>
              {[...(versions.Versions ?? [])].reverse().map((v) => (
                <li
                  key={v}
                  className={
                    "inline-flex items-center gap-1.5 rounded-md border px-2 py-1 " +
                    (v === versions.Active ? "border-primary/60 bg-accent" : "border-foreground/10")
                  }
                >
                  <span className="font-mono text-sm">{v}</span>
                  {v === versions.Active && (
                    <span className="rounded-sm bg-primary/15 px-1.5 text-[11px] text-accent-foreground">
                      {t("skills", "versionActive")}
                    </span>
                  )}
                </li>
              ))}
            </ul>
            <h3 className="text-xs text-muted-foreground">{t("skills", "changelogTitle")}</h3>
            {changelog.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("skills", "changelogEmpty")}</p>
            ) : (
              <ol className="flex flex-col gap-4">
                {changelog.map((e, i) => (
                  <li key={`${e.Heading}#${i}`} className="flex flex-col gap-1 border-l border-foreground/10 pl-4">
                    <span className="font-mono text-xs text-muted-foreground">
                      {e.At === ""
                        ? t("skills", "changelogUnreleased")
                        : [e.At, e.Version, e.Kind].filter((x) => x !== "").join(" · ")}
                    </span>
                    {e.Body.split("\n")
                      .map((l) => l.replace(/^\s*[-*]\s+/, "").trim())
                      .filter((l) => l !== "")
                      .map((l, j) => (
                        <span key={j} className="text-sm leading-relaxed">
                          {l}
                        </span>
                      ))}
                  </li>
                ))}
              </ol>
            )}
          </div>
        )}
      </Section>

      <Section label={t("skills", "usedByTitle")} title={t("skills", "usedByTitle")}>
        <p className="text-xs text-muted-foreground">{t("skills", "usedByNote")}</p>
        {connState !== undefined && connState !== "ready" ? (
          <p className="mt-2 text-sm text-muted-foreground">{t("skills", "usedByUnavailable")}</p>
        ) : usedByStale ? (
          <p className="mt-2 text-sm text-muted-foreground" role="status">
            {t("skills", "usedByLoading")}
          </p>
        ) : used.length === 0 ? (
          <p className="mt-2 text-sm text-muted-foreground">{t("skills", "usedByEmpty")}</p>
        ) : (
          <ul className="mt-3 flex flex-col gap-3 text-sm">
            {used.map((u) => {
              const wfs = u.workflows ?? [];
              return (
                <li key={u.componentId} className="flex flex-col gap-1">
                  <button
                    type="button"
                    onClick={() =>
                      navigate({
                        screen: "flow",
                        componentId: u.componentId,
                        ...(wfs.length > 0 ? { workflowId: wfs[0] } : {}),
                      })
                    }
                    className="desk-tint self-start font-mono text-xs hover:text-accent-foreground"
                  >
                    {u.componentId}
                  </button>
                  <div className="flex flex-wrap items-center gap-2 pl-5 text-xs text-muted-foreground">
                    {wfs.length === 0 ? (
                      <span>{t("skills", "usedByNoWorkflow")}</span>
                    ) : (
                      <>
                        <span>{t("skills", "usedByWorkflows")}</span>
                        {wfs.map((w) => (
                          <button
                            key={w}
                            type="button"
                            onClick={() => navigate({ screen: "flow", workflowId: w, componentId: u.componentId })}
                            className="desk-tint font-mono text-foreground/75 hover:text-foreground"
                          >
                            {w}
                          </button>
                        ))}
                      </>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Section>
    </div>
  );
}
