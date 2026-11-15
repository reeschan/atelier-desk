import { useEffect, useState } from "react";
import { DeskError, toDeskError, useClient } from "../api/client";
import type { ComponentView, WorkflowView } from "../api/types";
import { errorText, t } from "../i18n";
import { SaveBar } from "./SaveBar";
import { useHomeSave } from "./useHomeSave";
import type { TabProps } from "./useHomeSave";

const STAGES = [
  "understand",
  "design",
  "plan",
  "route",
  "build",
  "verify",
  "review",
  "fix",
  "finish",
] as const;

type Entry = string | { id: string; [k: string]: unknown };
type Data = Record<string, unknown>;
type WfEdit = { data: Data; source: string; bundled: boolean };
type Move = -1 | 1;

const FIELD = "h-8 rounded-md border border-foreground/10 bg-background px-2 text-xs";
const SMALL =
  "desk-tint h-8 rounded-md border border-foreground/15 px-2 text-xs hover:bg-muted disabled:opacity-40";
const CARD = "rounded-lg border border-foreground/10 bg-card p-5";

function entryId(e: Entry): string {
  return typeof e === "string" ? e : e.id;
}

function stagesOf(data: Data): Record<string, Entry[]> {
  return (data.stages ?? {}) as Record<string, Entry[]>;
}

function descriptionOf(data: Data): { ja: string; en: string } {
  const d = (data.description ?? {}) as { ja?: string; en?: string };
  return { ja: d.ja ?? "", en: d.en ?? "" };
}

function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}

/** The body for `workflow --name <name>`: everything the file holds, with the edited parts replaced. */
function serializeWorkflow(v: WfEdit): string {
  return JSON.stringify(v.data);
}

function titleOf(c: ComponentView): string {
  const title = c.component.data.title;
  const text = title?.ja ?? "";
  return text === "" || text === ">-" ? c.component.id : `${c.component.id} (${text})`;
}

// --- one stage's parts ---

function StageSection({
  stage,
  entries,
  components,
  onChange,
}: {
  stage: string;
  entries: Entry[];
  components: ComponentView[];
  onChange: (next: Entry[]) => void;
}) {
  const [pick, setPick] = useState("");
  const used = new Set(entries.map(entryId));
  const candidates = components.filter(
    (c) => (c.component.data.stages ?? []).includes(stage) && !used.has(c.component.id),
  );
  const chosen = candidates.some((c) => c.component.id === pick) ? pick : (candidates[0]?.component.id ?? "");

  function move(i: number, delta: Move) {
    const to = i + delta;
    if (to < 0 || to >= entries.length) return;
    const out = [...entries];
    const [m] = out.splice(i, 1);
    out.splice(to, 0, m);
    onChange(out);
  }

  return (
    <section aria-label={stage} data-stage={stage} className="rounded-md border border-foreground/10 p-3">
      <h4 className="font-mono text-sm">{stage}</h4>
      {entries.length === 0 && (
        <p className="mt-1 text-xs text-muted-foreground">{t("settings2", "wfStageEmpty")}</p>
      )}
      <ol className="mt-2 flex flex-col gap-1">
        {entries.map((e, i) => {
          const id = entryId(e);
          const label = `${stage} ${id}`;
          return (
            <li key={`${id}-${i}`} aria-label={label} className="flex items-center gap-2">
              <span className="w-6 font-mono text-xs text-muted-foreground">{i + 1}</span>
              <span className="font-mono text-xs break-all">{id}</span>
              {typeof e !== "string" && typeof e.when === "string" && (
                <span className="text-xs text-muted-foreground">when: {e.when}</span>
              )}
              <button
                type="button"
                className={SMALL}
                disabled={i === 0}
                aria-label={`${label} ${t("settings2", "itemUp")}`}
                onClick={() => move(i, -1)}
              >
                ↑
              </button>
              <button
                type="button"
                className={SMALL}
                disabled={i === entries.length - 1}
                aria-label={`${label} ${t("settings2", "itemDown")}`}
                onClick={() => move(i, 1)}
              >
                ↓
              </button>
              <button
                type="button"
                className={SMALL}
                aria-label={`${label} ${t("settings2", "itemRemove")}`}
                onClick={() => onChange(entries.filter((_, j) => j !== i))}
              >
                ✕
              </button>
            </li>
          );
        })}
      </ol>
      <div className="mt-2 flex items-center gap-2">
        <select
          className={`${FIELD} w-80 font-mono`}
          value={chosen}
          disabled={candidates.length === 0}
          aria-label={`${stage} ${t("settings2", "wfAddPick")}`}
          onChange={(e) => setPick(e.target.value)}
        >
          {candidates.map((c) => (
            <option key={c.component.id} value={c.component.id}>
              {titleOf(c)}
            </option>
          ))}
        </select>
        <button
          type="button"
          className={SMALL}
          disabled={chosen === ""}
          aria-label={`${stage} ${t("settings2", "wfAdd")}`}
          onClick={() => {
            onChange([...entries, chosen]);
            setPick("");
          }}
        >
          {t("settings2", "wfAdd")}
        </button>
        {candidates.length === 0 && (
          <span className="text-xs text-muted-foreground">{t("settings2", "wfNoCandidate")}</span>
        )}
      </div>
    </section>
  );
}

// --- one flow's form ---

function WorkflowEditor({
  name,
  ready,
  components,
  onChanged,
  onSelect,
}: {
  name: string;
  ready: boolean;
  components: ComponentView[];
  /** The set of flows changed (a save or a delete). */
  onChanged: () => void;
  onSelect: (name: string) => void;
}) {
  const client = useClient();
  const [copyName, setCopyName] = useState("");
  const [deleted, setDeleted] = useState(false);
  const h = useHomeSave<WfEdit>({
    kind: "workflow",
    name,
    ready,
    load: async () => {
      const [list, v] = await Promise.all([client.workflows(), client.homeState()]);
      const w = list.find((x) => x.workflow.name === name);
      if (w === undefined) throw new DeskError("not_found", name);
      // The form comes from workflows() and the base from homeState(), read side by side:
      // an outside edit between the two reads would be inside the base but not in the form.
      // Read the flow once more; if it moved meanwhile, the base cannot vouch for the form,
      // so its fingerprint of this flow's file is made stale and the next save comes back
      // as external_change instead of silently overwriting the outside edit.
      const again = (await client.workflows()).find((x) => x.workflow.name === name);
      const moved = again === undefined || JSON.stringify(again.workflow) !== JSON.stringify(w.workflow);
      const base = moved
        ? { ...v.base, Files: { ...(v.base.Files ?? {}), [`workflows/${name}.json`]: "changed-during-read" } }
        : v.base;
      return {
        value: {
          data: clone((w.workflow.data ?? {}) as Data),
          source: w.workflow.source,
          bundled: w.workflow.bundled,
        },
        base,
      };
    },
    serialize: serializeWorkflow,
    watches: (paths) => paths.some((p) => p.startsWith("workflows/") || p.startsWith("components/")),
  });
  const d = h.draft;
  const v = h.value;
  if (d === null || v === null) return null;

  function editData(fn: (data: Data) => Data) {
    h.setDraft((prev) => ({ ...prev, data: fn(prev.data) }));
  }

  const desc = descriptionOf(d.data);
  const stages = stagesOf(d.data);
  const mine = v.source === "home";
  const bar = {
    ...h,
    save: async () => {
      const r = await h.save();
      if (r !== null && r.Outcome === "saved") onChanged();
      return r;
    },
    overwrite: async () => {
      const r = await h.overwrite();
      if (r !== null && r.Outcome === "saved") onChanged();
      return r;
    },
  };

  async function remove() {
    const r = await h.save({ kind: "workflow-delete", name, input: "{}" });
    if (r === null || r.Outcome !== "saved") return;
    setDeleted(true);
    onChanged();
    if (!v?.bundled) onSelect("");
  }

  async function copyAs() {
    const target = copyName.trim();
    if (target === "" || d === null) return;
    const r = await h.save({
      kind: "workflow",
      name: target,
      input: serializeWorkflow({ ...d, data: { ...d.data, name: target } }),
    });
    if (r !== null && r.Outcome === "saved") {
      setCopyName("");
      onChanged();
      onSelect(target);
    }
  }

  return (
    <section aria-label={name} data-workflow-editor={name} className={CARD}>
      <h3 className="font-heading text-lg">{name}</h3>
      <p className="mt-1 text-xs text-muted-foreground">
        {mine
          ? v.bundled
            ? `${t("settings2", "wfSourceHome")}: ${t("settings2", "wfOverride")}`
            : t("settings2", "wfSourceHome")
          : `${t("settings2", "wfSourceBundled")}: ${t("settings2", "wfBundledOnly")}`}
      </p>
      <div className="mt-3 flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "wfDescJa")}
          <textarea
            className="min-h-16 rounded-md border border-foreground/10 bg-background p-2 text-xs"
            value={desc.ja}
            aria-label={t("settings2", "wfDescJa")}
            onChange={(e) => editData((x) => ({ ...x, description: { ...descriptionOf(x), ja: e.target.value } }))}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "wfDescEn")}
          <textarea
            className="min-h-16 rounded-md border border-foreground/10 bg-background p-2 text-xs"
            value={desc.en}
            aria-label={t("settings2", "wfDescEn")}
            onChange={(e) => editData((x) => ({ ...x, description: { ...descriptionOf(x), en: e.target.value } }))}
          />
        </label>
        <h4 className="text-sm">{t("settings2", "wfStagesTitle")}</h4>
        {STAGES.map((stage) => (
          <StageSection
            key={stage}
            stage={stage}
            entries={stages[stage] ?? []}
            components={components}
            onChange={(next) => editData((x) => ({ ...x, stages: { ...stagesOf(x), [stage]: next } }))}
          />
        ))}
      </div>
      <SaveBar save={bar} ready={ready} />
      <div className="mt-4 border-t border-foreground/10 pt-3">
        <h4 className="text-sm">{t("settings2", "wfCopyTitle")}</h4>
        <div className="mt-2 flex items-center gap-2">
          <input
            type="text"
            className={`${FIELD} w-64 font-mono`}
            value={copyName}
            aria-label={t("settings2", "wfCopyName")}
            placeholder={t("settings2", "wfCopyName")}
            onChange={(e) => setCopyName(e.target.value)}
          />
          <button
            type="button"
            className={SMALL}
            disabled={!ready || copyName.trim() === ""}
            onClick={() => void copyAs()}
          >
            {t("settings2", "wfCopy")}
          </button>
        </div>
      </div>
      <div className="mt-4 border-t border-foreground/10 pt-3">
        <h4 className="text-sm">{t("settings2", "wfDeleteTitle")}</h4>
        <p className="mt-1 text-xs text-muted-foreground" data-delete-note>
          {!mine
            ? t("settings2", "wfDeleteNone")
            : v.bundled
              ? t("settings2", "wfDeleteBack")
              : t("settings2", "wfDeleteGone")}
        </p>
        <button
          type="button"
          className={`${SMALL} mt-2`}
          disabled={!ready || !mine}
          onClick={() => void remove()}
        >
          {t("settings2", "wfDelete")}
        </button>
        {deleted && <p className="mt-1 text-xs text-success">{t("settings2", "wfDeleted")}</p>}
      </div>
    </section>
  );
}

// --- the tab ---

type Load =
  | { state: "loading" }
  | { state: "error"; code: string }
  | { state: "ready"; workflows: WorkflowView[]; components: ComponentView[] };

/** The flows: pick one, edit its description and the parts of each stage, copy it, delete one's own (user story 24). */
export function WorkflowsTab({ ready }: TabProps) {
  const client = useClient();
  const [load, setLoad] = useState<Load>({ state: "loading" });
  const [rev, setRev] = useState(0);
  const [selected, setSelected] = useState("");

  useEffect(() => {
    let live = true;
    Promise.all([client.workflows(), client.components()])
      .then(([workflows, components]) => {
        if (live) setLoad({ state: "ready", workflows, components });
      })
      .catch((e: unknown) => {
        if (live) setLoad({ state: "error", code: toDeskError(e).code });
      });
    return () => {
      live = false;
    };
  }, [client, rev, ready]);

  useEffect(
    () => client.events.onHomeChanged(() => setRev((n) => n + 1)),
    [client],
  );

  if (load.state === "loading") return <p className="text-xs text-muted-foreground">{t("settings2", "wfLoading")}</p>;
  if (load.state === "error") {
    return (
      <p role="alert" className="text-xs text-destructive">
        {t("settings2", "wfLoadFailed")}: {errorText(load.code)}
      </p>
    );
  }
  const { workflows, components } = load;
  const names = workflows.map((w) => w.workflow.name);
  const current = names.includes(selected) ? selected : (names[0] ?? "");

  return (
    <div className="flex flex-col gap-4">
      <section aria-label={t("settings2", "wfListTitle")} className={CARD}>
        <h3 className="font-heading text-lg">{t("settings2", "wfListTitle")}</h3>
        {workflows.length === 0 && <p className="mt-2 text-xs text-muted-foreground">{t("settings2", "wfEmpty")}</p>}
        <ul className="mt-3 flex flex-col gap-1">
          {workflows.map((w) => {
            const n = w.workflow.name;
            return (
              <li key={n} data-workflow={n} className="flex items-center gap-2">
                <button
                  type="button"
                  className={SMALL}
                  aria-pressed={n === current}
                  onClick={() => setSelected(n)}
                >
                  {n}
                </button>
                <span className="text-xs text-muted-foreground">
                  {w.workflow.source === "home" ? t("settings2", "wfSourceHome") : t("settings2", "wfSourceBundled")}
                </span>
              </li>
            );
          })}
        </ul>
      </section>
      {current !== "" && (
        <WorkflowEditor
          key={current}
          name={current}
          ready={ready}
          components={components}
          onChanged={() => setRev((n) => n + 1)}
          onSelect={(n) => setSelected(n)}
        />
      )}
    </div>
  );
}
