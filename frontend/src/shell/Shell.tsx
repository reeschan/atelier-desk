import { useState } from "react";
import type { FormEvent } from "react";
import { toDeskError, useClient } from "../api/client";
import type { Connection } from "../api/types";
import { errorText, t } from "../i18n";
import { FlowScreen } from "../screens/FlowScreen";
import { RunsScreen } from "../screens/RunsScreen";
import { SettingsScreen } from "../screens/SettingsScreen";
import { SkillsScreen } from "../screens/SkillsScreen";
import { ConnectionBadge, useConnection } from "./ConnectionBadge";
import { toHash, useRoute } from "./routes";
import type { Route } from "./routes";
import { VersionPopover } from "./VersionPopover";

/**
 * Motion of the shell (docs/pbi/ui/atelier-desk/README.md): spring-press on the
 * main button 260ms, entry-transition of the version popover 200ms, scale-in of
 * the state badge 220ms once. All of it stops for prefers-reduced-motion.
 */
const SHELL_CSS = `
:root { --desk-ease: cubic-bezier(0.22, 1, 0.36, 1); }
.desk-tint { transition: background-color 150ms var(--desk-ease), color 150ms var(--desk-ease), border-color 150ms var(--desk-ease); }
.desk-press { transition: transform 260ms linear(0, 0.42 12%, 0.78 28%, 0.96 44%, 1.012 60%, 1.003 78%, 1), background-color 150ms var(--desk-ease); }
.desk-press:active { transform: scale(0.97); transition-duration: 100ms, 150ms; transition-timing-function: var(--desk-ease), var(--desk-ease); }
@keyframes desk-pop-in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
.desk-pop { animation: desk-pop-in 200ms var(--desk-ease) 1 both; }
@keyframes desk-badge-in { from { opacity: 0; transform: scale(0.86); } to { opacity: 1; transform: none; } }
.desk-badge-in { animation: desk-badge-in 220ms var(--desk-ease) 1 both; transform-origin: left center; }
@media (prefers-reduced-motion: reduce) {
  .desk-press, .desk-press:active { transition: none; transform: none; }
  .desk-pop { animation: none; }
  .desk-badge-in { animation: none; }
  .desk-tint { transition: none; }
}
`;

type Screen = Route["screen"];

const NAV: { screen: Screen; label: string; icon: string[] }[] = [
  {
    screen: "flow",
    label: "navFlow",
    icon: ["M3 3h8v8H3z", "M7 11v4a2 2 0 0 0 2 2h4", "M13 13h8v8h-8z"],
  },
  {
    screen: "skills",
    label: "navSkills",
    icon: [
      "M12 7v14",
      "M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z",
    ],
  },
  {
    screen: "settings",
    label: "navSettings",
    icon: ["M21 4h-7M10 4H3M21 12h-9M8 12H3M21 20h-5M12 20H3M14 2v4M8 10v4M16 18v4"],
  },
  {
    screen: "runs",
    label: "navRuns",
    icon: ["M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8", "M3 3v5h5", "M12 7v5l4 2"],
  },
];

function NavIcon({ paths }: { paths: string[] }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="shrink-0"
      aria-hidden="true"
    >
      {paths.map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}

function ScreenBody({ route }: { route: Route }) {
  switch (route.screen) {
    case "flow":
      return <FlowScreen />;
    case "skills":
      return <SkillsScreen />;
    case "settings":
      return <SettingsScreen />;
    case "runs":
      return <RunsScreen />;
  }
}

/** What is missing and how to fix it, shown over the body when the connection is not ready. */
function ConnectionBanner({
  conn,
  onChanged,
}: {
  conn: Connection;
  onChanged: (c: Connection) => void;
}) {
  const client = useClient();
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState("");

  async function apply(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setFailed("");
    try {
      onChanged(await client.setPythonPath(path.trim()));
    } catch (err) {
      setFailed(errorText(toDeskError(err).code));
    } finally {
      setBusy(false);
    }
  }

  if (conn.state === "checking") {
    return (
      <section aria-label={t("common", "bannerReadyLabel")} className="mb-6 max-w-3xl">
        <p className="text-sm text-muted-foreground">{t("common", "bannerChecking")}</p>
      </section>
    );
  }

  const python = conn.state === "python_unavailable";
  return (
    <section aria-label={t("common", "bannerReadyLabel")} className="mb-8 max-w-3xl">
      <p className="text-sm text-destructive">
        {t("common", python ? "bannerPythonLead" : "bannerUnsupportedLead")}
      </p>
      <h1 className="mt-2 font-heading text-3xl">
        {errorText(conn.code === "" ? (python ? "python_not_found" : "unsupported") : conn.code)}
      </h1>
      <p className="mt-2 text-sm leading-relaxed text-foreground/75">
        {t("common", python ? "bannerPythonBody" : "bannerUnsupportedBody")}
      </p>
      {conn.diagnostic !== "" && (
        <div className="mt-6 rounded-lg border border-foreground/10 bg-card p-5">
          <h2 className="text-sm">{t("common", "diagnosticLabel")}</h2>
          <pre className="mt-3 font-mono text-xs break-words whitespace-pre-wrap text-foreground/75">
            {conn.diagnostic}
          </pre>
        </div>
      )}
      {python && (
        <ol className="mt-6 flex flex-col gap-5">
          <li className="grid grid-cols-[1.75rem_1fr] gap-3">
            <span className="font-mono text-sm text-muted-foreground">1</span>
            <p className="text-sm">{t("common", "fixInstallPython")}</p>
          </li>
          <li className="grid grid-cols-[1.75rem_1fr] gap-3">
            <span className="font-mono text-sm text-muted-foreground">2</span>
            <form onSubmit={(e) => void apply(e)} className="flex flex-col gap-2">
              <p className="text-sm">{t("common", "fixSetPath")}</p>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder={t("common", "pythonPathPlaceholder")}
                  aria-label={t("common", "pythonPathLabel")}
                  className="h-9 w-[30rem] max-w-full rounded-md border border-foreground/10 bg-background px-3 font-mono text-xs"
                />
                <button
                  type="submit"
                  disabled={busy}
                  className="desk-press inline-flex h-9 items-center rounded-md bg-primary px-4 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                >
                  {t("common", "pythonPathApply")}
                </button>
              </div>
              {failed !== "" && (
                <p role="alert" className="text-xs text-destructive">
                  {t("common", "pythonPathFailed")}: {failed}
                </p>
              )}
              <p className="text-xs text-muted-foreground">{t("common", "pythonPathNote")}</p>
            </form>
          </li>
        </ol>
      )}
    </section>
  );
}

export function Shell() {
  const route = useRoute();
  const [conn, setConn, beginSwitch] = useConnection();
  const state = conn?.state ?? "checking";
  const version = conn?.version ?? "";

  let note = "";
  if (state === "checking") note = t("common", "checkingNote");
  else if (state === "ready" && conn !== null && conn.pythonVersion !== "") {
    note = t("common", "pythonVersion", { version: conn.pythonVersion });
  }

  return (
    <div className="flex min-h-screen flex-col">
      <style>{SHELL_CSS}</style>
      <header className="flex h-14 items-center gap-4 border-b border-foreground/10 px-4">
        <a href={toHash({ screen: "flow" })} className="flex items-center gap-2 font-heading text-base text-foreground">
          <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true">
            <path
              d="M2 7h16M5 7v9M15 7v9M7 4h6"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
            />
          </svg>
          {t("common", "appName")}
        </a>
        <span className="h-5 w-px bg-foreground/10" aria-hidden="true" />
        <VersionPopover
          version={version}
          disabled={state === "checking"}
          onSwitched={setConn}
          onSwitchStart={beginSwitch}
        />
        <div className="ml-auto flex items-center gap-3">
          {note !== "" && <span className="font-mono text-xs text-muted-foreground">{note}</span>}
          <ConnectionBadge state={state} />
        </div>
      </header>
      <div className="flex flex-1">
        <aside className="flex w-56 shrink-0 flex-col border-r border-foreground/10 px-3 py-4">
          <nav className="flex flex-col gap-1" aria-label={t("common", "navLabel")}>
            {NAV.map((n) => {
              const current = route.screen === n.screen;
              return (
                <a
                  key={n.screen}
                  href={toHash({ screen: n.screen } as Route)}
                  aria-current={current ? "page" : undefined}
                  className={`desk-tint flex h-9 items-center gap-2.5 rounded-md px-2.5 text-sm ${
                    current
                      ? "bg-card text-foreground"
                      : "text-muted-foreground hover:bg-card/60 hover:text-foreground"
                  }`}
                >
                  <NavIcon paths={n.icon} />
                  {t("common", n.label)}
                </a>
              );
            })}
          </nav>
          {conn !== null && conn.home !== "" && (
            <div className="mt-auto flex flex-col gap-1 px-2.5 text-xs text-muted-foreground">
              <span>home</span>
              <span className="font-mono break-all">{conn.home}</span>
            </div>
          )}
        </aside>
        <main className="min-w-0 flex-1 px-8 py-8">
          {conn !== null && conn.state !== "ready" && (
            <ConnectionBanner conn={conn} onChanged={setConn} />
          )}
          <ScreenBody route={route} />
        </main>
      </div>
    </div>
  );
}
