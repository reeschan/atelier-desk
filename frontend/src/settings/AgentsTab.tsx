import { useState } from "react";
import { DeskError, useClient } from "../api/client";
import type { HomeState } from "../api/types";
import { t } from "../i18n";
import { SaveBar } from "./SaveBar";
import { useHomeSave } from "./useHomeSave";
import type { TabProps } from "./useHomeSave";

type RunnerState = HomeState["runners"][string];

type ModelRow = { id: string; name: string; efforts: string[]; manual: boolean };

/** What one runner card edits (enabled, selected, billing) and shows (the rest). */
type RunnerEdit = {
  enabled: boolean;
  selected: string[] | null;
  billing: string;
  path: string;
  version: string;
  checkedAt: string;
  models: ModelRow[];
  fetchedAt: string;
  method: string;
  fetchError: string;
  missing: string[];
};

function fromRunner(r: RunnerState): RunnerEdit {
  return {
    enabled: r.enabled,
    selected: r.selected ?? null,
    billing: r.billing ?? "",
    path: r.path ?? "",
    version: r.version ?? "",
    checkedAt: r.checked_at ?? "",
    models: (r.models?.list ?? []).map((m) => ({
      id: m.id,
      name: m.name ?? "",
      efforts: m.efforts ?? [],
      manual: m.manual === true,
    })),
    fetchedAt: r.models?.fetched_at ?? "",
    method: r.models?.method ?? "",
    fetchError: r.models?.error ?? "",
    missing: r.missing_selected ?? [],
  };
}

/** The body for `runner --name <runner>`: only what the card edits. */
function serializeRunner(v: RunnerEdit): string {
  const body: { enabled: boolean; selected: string[] | null; billing?: string } = {
    enabled: v.enabled,
    selected: v.selected,
  };
  if (v.billing !== "") body.billing = v.billing;
  return JSON.stringify(body);
}

function splitEfforts(text: string): string[] {
  return text
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s !== "");
}

function RunnerCard({ name, ready }: { name: string; ready: boolean }) {
  const client = useClient();
  const h = useHomeSave<RunnerEdit>({
    kind: "runner",
    name,
    ready,
    load: async () => {
      const v = await client.homeState();
      const r = v.state.runners[name];
      if (r === undefined) throw new DeskError("not_found", name);
      return { value: fromRunner(r), base: v.base };
    },
    serialize: serializeRunner,
    watches: (paths) => paths.some((p) => p === "agents.json" || p === `models/${name}.json`),
  });
  const [filter, setFilter] = useState("");
  const [modelId, setModelId] = useState("");
  const [efforts, setEfforts] = useState("");

  const d = h.draft;
  if (d === null) return null;
  const always = name === "agent";
  const all = d.selected === null;
  const shown = d.models.filter(
    (m) => filter === "" || m.id.includes(filter) || m.name.includes(filter),
  );

  function toggle(id: string, on: boolean) {
    h.setDraft((prev) => {
      const base = prev.selected ?? prev.models.map((m) => m.id);
      const next = on ? (base.includes(id) ? base : [...base, id]) : base.filter((x) => x !== id);
      return { ...prev, selected: next };
    });
  }

  async function addModel() {
    const r = await h.save({
      kind: "model",
      name,
      input: JSON.stringify({ id: modelId.trim(), efforts: splitEfforts(efforts) }),
    });
    if (r !== null && r.Outcome === "saved") {
      setModelId("");
      setEfforts("");
    }
  }

  const canAdd = ready && h.state === "viewing" && !h.changed && modelId.trim() !== "";

  return (
    <article
      aria-label={name}
      data-runner={name}
      className="rounded-lg border border-foreground/10 bg-card p-5"
    >
      <header className="flex items-center justify-between gap-3">
        <h3 className="font-heading text-lg">{name}</h3>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={always ? true : d.enabled}
            disabled={always}
            onChange={(e) => h.setDraft((prev) => ({ ...prev, enabled: e.target.checked }))}
            aria-label={`${name} ${t("settings", "runnerEnabled")}`}
          />
          {always ? t("settings", "runnerAlwaysOn") : t("settings", "runnerEnabled")}
        </label>
      </header>

      <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
        {!always && (
          <>
            <dt className="text-muted-foreground">{t("settings", "runnerPath")}</dt>
            <dd className="font-mono break-all">
              {d.path === "" ? t("settings", "runnerMissingPath") : d.path}
            </dd>
            <dt className="text-muted-foreground">{t("settings", "runnerVersion")}</dt>
            <dd className="font-mono">{d.version}</dd>
            <dt className="text-muted-foreground">{t("settings", "runnerCheckedAt")}</dt>
            <dd className="font-mono">{d.checkedAt}</dd>
          </>
        )}
        <dt className="text-muted-foreground">{t("settings", "modelsFetched")}</dt>
        <dd className="font-mono">{d.fetchedAt}</dd>
        <dt className="text-muted-foreground">{t("settings", "modelsMethod")}</dt>
        <dd className="font-mono">{d.method}</dd>
      </dl>
      {d.fetchError !== "" && (
        <p role="alert" className="mt-2 text-xs text-warning">
          {t("settings", "modelsError")}: {d.fetchError}
        </p>
      )}

      <label className="mt-4 flex items-center gap-3 text-sm">
        <span className="text-muted-foreground">{t("settings", "billing")}</span>
        <input
          type="text"
          value={d.billing}
          placeholder={t("settings", "billingPlaceholder")}
          onChange={(e) => h.setDraft((prev) => ({ ...prev, billing: e.target.value }))}
          className="h-8 w-64 rounded-md border border-foreground/10 bg-background px-2 font-mono text-xs"
        />
      </label>

      <div className="mt-4">
        <div className="flex items-center justify-between gap-3">
          <h4 className="text-sm">{t("settings", "modelsTitle")}</h4>
          <div className="flex items-center gap-2">
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              aria-label={t("settings", "modelFilter")}
              placeholder={t("settings", "modelFilter")}
              className="h-8 w-48 rounded-md border border-foreground/10 bg-background px-2 text-xs"
            />
            <button
              type="button"
              disabled={all}
              onClick={() => h.setDraft((prev) => ({ ...prev, selected: null }))}
              className="desk-tint h-8 rounded-md border border-foreground/15 px-3 text-xs hover:bg-muted disabled:opacity-50"
            >
              {t("settings", "modelsAll")}
            </button>
          </div>
        </div>
        {all && <p className="mt-1 text-xs text-muted-foreground">{t("settings", "modelsAllNote")}</p>}
        {d.missing.length > 0 && (
          <p role="alert" className="mt-2 text-xs text-warning">
            {t("settings", "missingSelected")}: <span className="font-mono">{d.missing.join(", ")}</span>
          </p>
        )}
        {d.models.length === 0 ? (
          <p className="mt-2 text-xs text-muted-foreground">{t("settings", "modelsEmpty")}</p>
        ) : (
          <ul className="mt-2 flex max-h-72 flex-col gap-1 overflow-y-auto">
            {shown.map((m) => (
              <li key={m.id}>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={all || (d.selected ?? []).includes(m.id)}
                    onChange={(e) => toggle(m.id, e.target.checked)}
                  />
                  <span className="font-mono text-xs">{m.id}</span>
                  {m.name !== "" && <span className="text-muted-foreground">{m.name}</span>}
                  {m.manual && (
                    <span className="rounded-sm bg-muted px-1.5 text-xs">
                      {t("settings", "modelManual")}
                    </span>
                  )}
                  {m.efforts.length > 0 && (
                    <span className="font-mono text-xs text-muted-foreground">
                      {t("settings", "modelEfforts")} {m.efforts.join("/")}
                    </span>
                  )}
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>

      <form
        className="mt-4 flex flex-wrap items-center gap-2 border-t border-foreground/10 pt-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canAdd) void addModel();
        }}
      >
        <span className="text-sm">{t("settings", "addModelTitle")}</span>
        <input
          type="text"
          value={modelId}
          onChange={(e) => setModelId(e.target.value)}
          aria-label={t("settings", "addModelId")}
          placeholder={t("settings", "addModelId")}
          className="h-8 w-56 rounded-md border border-foreground/10 bg-background px-2 font-mono text-xs"
        />
        <input
          type="text"
          value={efforts}
          onChange={(e) => setEfforts(e.target.value)}
          aria-label={t("settings", "addModelEfforts")}
          placeholder={t("settings", "addModelEfforts")}
          className="h-8 w-48 rounded-md border border-foreground/10 bg-background px-2 font-mono text-xs"
        />
        <button
          type="submit"
          disabled={!canAdd}
          className="desk-tint h-8 rounded-md border border-foreground/15 px-3 text-sm hover:bg-muted disabled:opacity-50"
        >
          {t("settings", "addModelButton")}
        </button>
        {h.changed && (
          <span className="text-xs text-muted-foreground">{t("settings", "addModelBlocked")}</span>
        )}
      </form>

      <SaveBar save={h} ready={ready} />
    </article>
  );
}

/** One card per runner (user story 19, 20). */
export function AgentsTab({ state, ready }: TabProps) {
  const names = Object.keys(state.runners).sort();
  if (names.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("settings", "agentsEmpty")}</p>;
  }
  return (
    <div className="flex flex-col gap-4">
      {names.map((n) => (
        <RunnerCard key={n} name={n} ready={ready} />
      ))}
    </div>
  );
}
