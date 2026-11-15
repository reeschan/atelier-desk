import { useClient } from "../api/client";
import type { HomeState } from "../api/types";
import { t } from "../i18n";
import { SaveBar } from "./SaveBar";
import { useHomeSave } from "./useHomeSave";
import type { TabProps } from "./useHomeSave";

const TIERS = ["haiku", "sonnet", "opus"] as const;
type Tier = (typeof TIERS)[number];

type StepEdit = { runner: string; model: string; effort: string; until: string };
type TierEdit = { external: StepEdit[]; fixed: StepEdit };
type RunnerOption = { name: string; models: string[] };

/** What the routing form edits (source, name, tiers) and the options it offers (runners). */
type RoutingEdit = {
  source: string;
  name: string;
  tiers: Record<Tier, TierEdit>;
  runners: RunnerOption[];
  /** The home routing.json's own description, kept as read; the form has no field for it. */
  description?: unknown;
};

type StateStep = { runner: string; model: string; effort?: string | null; until?: string | null };

function toEdit(s: StateStep): StepEdit {
  return { runner: s.runner, model: s.model, effort: s.effort ?? "", until: s.until ?? "" };
}

function fromState(state: HomeState): RoutingEdit {
  const table = state.routing.table;
  const tiers = {} as Record<Tier, TierEdit>;
  for (const tier of TIERS) {
    const steps = (table.tiers?.[tier] ?? []).map(toEdit);
    const last = steps[steps.length - 1];
    if (last !== undefined && last.runner === "agent") {
      tiers[tier] = { external: steps.slice(0, -1), fixed: last };
    } else {
      tiers[tier] = { external: steps, fixed: { runner: "agent", model: tier, effort: "", until: "" } };
    }
  }
  const runners = Object.keys(state.runners)
    .sort()
    .filter((n) => n !== "agent" && state.runners[n].enabled)
    .map((n) => {
      const r = state.runners[n];
      const listed = (r.models?.list ?? []).map((m) => m.id);
      return { name: n, models: r.selected ?? listed };
    });
  const edit: RoutingEdit = { source: state.routing.source, name: table.name ?? "", tiers, runners };
  const description = (table as { description?: unknown }).description;
  if (state.routing.source === "home" && description !== undefined && description !== null) {
    edit.description = description;
  }
  return edit;
}

function stepBody(s: StepEdit): Record<string, unknown> {
  const body: Record<string, unknown> = {
    runner: s.runner,
    model: s.model,
    effort: s.effort === "" ? null : s.effort,
  };
  if (s.until !== "") body.until = s.until;
  return body;
}

/** The body for `routing`: three tiers, each the external steps and then the fixed agent step. */
function serializeRouting(v: RoutingEdit): string {
  const tiers: Record<string, Record<string, unknown>[]> = {};
  for (const tier of TIERS) {
    const { external, fixed } = v.tiers[tier];
    tiers[tier] = [...external.map(stepBody), stepBody({ ...fixed, until: "" })];
  }
  const body: { tiers: typeof tiers; name?: string; description?: unknown } = { tiers };
  if (v.source === "home" && v.name !== "") body.name = v.name;
  if (v.source === "home" && v.description !== undefined) body.description = v.description;
  return JSON.stringify(body);
}

const FIELD = "h-8 rounded-md border border-foreground/10 bg-background px-2 text-xs";
const SMALL =
  "desk-tint h-8 rounded-md border border-foreground/15 px-2 text-xs hover:bg-muted disabled:opacity-40";

function withCurrent(options: string[], current: string): string[] {
  return current === "" || options.includes(current) ? options : [current, ...options];
}

function StepRow({
  tier,
  index,
  last,
  step,
  runners,
  onChange,
  onMove,
  onRemove,
}: {
  tier: Tier;
  index: number;
  last: boolean;
  step: StepEdit;
  runners: RunnerOption[];
  onChange: (s: StepEdit) => void;
  onMove: (delta: -1 | 1) => void;
  onRemove: () => void;
}) {
  const label = `${tier} ${t("settings", "stepNumber", { n: index + 1 })}`;
  const runnerNames = withCurrent(
    runners.map((r) => r.name),
    step.runner,
  );
  const models = withCurrent(runners.find((r) => r.name === step.runner)?.models ?? [], step.model);
  return (
    <li className="flex flex-wrap items-center gap-2" aria-label={label}>
      <span className="w-6 font-mono text-xs text-muted-foreground">{index + 1}</span>
      <select
        className={FIELD}
        value={step.runner}
        aria-label={`${label} ${t("settings", "stepRunner")}`}
        onChange={(e) => {
          const next = runners.find((r) => r.name === e.target.value);
          onChange({ ...step, runner: e.target.value, model: next?.models[0] ?? "" });
        }}
      >
        {runnerNames.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>
      {models.length > 0 ? (
        <select
          className={`${FIELD} w-64 font-mono`}
          value={step.model}
          aria-label={`${label} ${t("settings", "stepModel")}`}
          onChange={(e) => onChange({ ...step, model: e.target.value })}
        >
          {models.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      ) : (
        <input
          type="text"
          className={`${FIELD} w-64 font-mono`}
          value={step.model}
          aria-label={`${label} ${t("settings", "stepModel")}`}
          onChange={(e) => onChange({ ...step, model: e.target.value })}
        />
      )}
      <input
        type="text"
        className={`${FIELD} w-24 font-mono`}
        value={step.effort}
        placeholder={t("settings", "stepEffort")}
        aria-label={`${label} ${t("settings", "stepEffort")}`}
        onChange={(e) => onChange({ ...step, effort: e.target.value })}
      />
      <input
        type="date"
        className={`${FIELD} font-mono`}
        value={step.until}
        aria-label={`${label} ${t("settings", "stepUntil")}`}
        onChange={(e) => onChange({ ...step, until: e.target.value })}
      />
      <button
        type="button"
        className={SMALL}
        disabled={index === 0}
        aria-label={`${label} ${t("settings", "stepUp")}`}
        onClick={() => onMove(-1)}
      >
        ↑
      </button>
      <button
        type="button"
        className={SMALL}
        disabled={last}
        aria-label={`${label} ${t("settings", "stepDown")}`}
        onClick={() => onMove(1)}
      >
        ↓
      </button>
      <button
        type="button"
        className={SMALL}
        aria-label={`${label} ${t("settings", "stepRemove")}`}
        onClick={onRemove}
      >
        ×
      </button>
    </li>
  );
}

/** The three tiers' steps (user story 21). */
export function RoutingTab({ ready }: TabProps) {
  const client = useClient();
  const h = useHomeSave<RoutingEdit>({
    kind: "routing",
    ready,
    load: async () => {
      const v = await client.homeState();
      return { value: fromState(v.state), base: v.base };
    },
    serialize: serializeRouting,
    watches: (paths) => paths.some((p) => p === "routing.json" || p === "agents.json"),
  });
  const d = h.draft;
  if (d === null) return null;

  function editTier(tier: Tier, fn: (steps: StepEdit[]) => StepEdit[]) {
    h.setDraft((prev) => ({
      ...prev,
      tiers: { ...prev.tiers, [tier]: { ...prev.tiers[tier], external: fn(prev.tiers[tier].external) } },
    }));
  }

  function add(tier: Tier) {
    const first = d?.runners[0];
    if (first === undefined) return;
    editTier(tier, (steps) => [
      ...steps,
      { runner: first.name, model: first.models[0] ?? "", effort: "", until: "" },
    ]);
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-xs text-muted-foreground">
        {t("settings", d.source === "home" ? "routingHome" : "routingDefault")}
      </p>
      {TIERS.map((tier) => {
        const { external, fixed } = d.tiers[tier];
        return (
          <section
            key={tier}
            aria-label={t("settings", tier === "haiku" ? "tierHaiku" : tier === "sonnet" ? "tierSonnet" : "tierOpus")}
            data-tier={tier}
            className="rounded-lg border border-foreground/10 bg-card p-5"
          >
            <h3 className="font-heading text-lg">{tier}</h3>
            {external.length === 0 && (
              <p className="mt-2 text-xs text-muted-foreground">{t("settings", "tierEmptyNote")}</p>
            )}
            <ol className="mt-3 flex flex-col gap-2">
              {external.map((s, i) => (
                <StepRow
                  key={i}
                  tier={tier}
                  index={i}
                  last={i === external.length - 1}
                  step={s}
                  runners={d.runners}
                  onChange={(next) => editTier(tier, (steps) => steps.map((x, j) => (j === i ? next : x)))}
                  onMove={(delta) =>
                    editTier(tier, (steps) => {
                      const out = [...steps];
                      const [moved] = out.splice(i, 1);
                      out.splice(i + delta, 0, moved);
                      return out;
                    })
                  }
                  onRemove={() => editTier(tier, (steps) => steps.filter((_, j) => j !== i))}
                />
              ))}
            </ol>
            <div className="mt-3 flex items-center gap-3">
              <button type="button" className={SMALL} disabled={d.runners.length === 0} onClick={() => add(tier)}>
                {t("settings", "stepAdd")}
              </button>
              {d.runners.length === 0 && (
                <span className="text-xs text-muted-foreground">{t("settings", "stepNoRunner")}</span>
              )}
            </div>
            <div className="mt-3 border-t border-foreground/10 pt-3" data-fixed-step>
              <p className="text-xs text-muted-foreground">
                {t("settings", "fixedStep")}: {t("settings", "fixedNote")}
              </p>
              <p className="mt-1 font-mono text-xs">
                {fixed.runner} / {fixed.model}
                {fixed.effort !== "" ? ` / ${fixed.effort}` : ""}
              </p>
            </div>
          </section>
        );
      })}
      <SaveBar save={h} ready={ready} />
    </div>
  );
}
