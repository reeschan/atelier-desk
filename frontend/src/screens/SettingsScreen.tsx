import { useEffect, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { HomeView } from "../api/types";
import { errorText, t } from "../i18n";
import { AgentsTab } from "../settings/AgentsTab";
import { PriorityTab } from "../settings/PriorityTab";
import { RoutingTab } from "../settings/RoutingTab";
import { WorkflowsTab } from "../settings/WorkflowsTab";
import { useConnection } from "../shell/ConnectionBadge";
import { navigate, useRoute } from "../shell/routes";
import type { SettingsTab } from "../shell/routes";

const TABS: { id: SettingsTab; label: string }[] = [
  { id: "agents", label: "tabAgents" },
  { id: "routing", label: "tabRouting" },
  { id: "priority", label: "tabPriority" },
  { id: "workflows", label: "tabWorkflows" },
];

/** The four tabs of `studio` as forms. Each tab owns its save (useHomeSave); this frame reads the home and says the limit of a save. */
export function SettingsScreen() {
  const client = useClient();
  const route = useRoute();
  const [conn] = useConnection();
  const ready = conn?.state === "ready";
  const tab: SettingsTab = route.screen === "settings" && route.tab !== undefined ? route.tab : "agents";
  const [view, setView] = useState<HomeView | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    function load() {
      client
        .homeState()
        .then((v) => {
          if (!live) return;
          setView(v);
          setError("");
        })
        .catch((e: unknown) => {
          if (live) setError(toDeskError(e).code);
        });
    }
    load();
    const off = client.events.onHomeChanged(load);
    return () => {
      live = false;
      off();
    };
  }, [client, ready]);

  return (
    <div className="max-w-4xl">
      <h1 className="font-heading text-3xl">{t("settings", "title")}</h1>
      <p className="mt-2 text-xs text-muted-foreground" data-save-limit>
        {t("settings", "limitNote")}
      </p>
      <div role="tablist" aria-label={t("settings", "tabsLabel")} className="mt-6 flex gap-1 border-b border-foreground/10">
        {TABS.map((x) => (
          <button
            key={x.id}
            type="button"
            role="tab"
            id={`settings-tab-${x.id}`}
            aria-selected={tab === x.id}
            aria-controls="settings-panel"
            onClick={() => navigate({ screen: "settings", tab: x.id })}
            className={`desk-tint h-9 rounded-t-md px-4 text-sm ${
              tab === x.id ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-muted"
            }`}
          >
            {t("settings", x.label)}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id="settings-panel"
        aria-labelledby={`settings-tab-${tab}`}
        className="mt-6"
      >
        {view === null ? (
          <p className="text-sm text-muted-foreground">
            {error === ""
              ? t("settings", "loading")
              : `${t("settings", "loadFailed")}: ${errorText(error)}`}
          </p>
        ) : (
          <>
            {(view.state.warnings ?? []).length > 0 && (
              <section
                aria-label={t("settings", "warningsTitle")}
                className="mb-4 rounded-md border border-warning/50 p-3"
              >
                <ul className="flex flex-col gap-1">
                  {(view.state.warnings ?? []).map((w, i) => (
                    <li key={i} className="text-xs break-words">
                      {w}
                    </li>
                  ))}
                </ul>
              </section>
            )}
            {tab === "agents" && <AgentsTab state={view.state} base={view.base} ready={ready} />}
            {tab === "routing" && <RoutingTab state={view.state} base={view.base} ready={ready} />}
            {tab === "priority" && <PriorityTab state={view.state} base={view.base} ready={ready} />}
            {tab === "workflows" && (
              <WorkflowsTab state={view.state} base={view.base} ready={ready} />
            )}
          </>
        )}
      </div>
    </div>
  );
}
