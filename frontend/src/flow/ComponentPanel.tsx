import type { ComponentView } from "../api/types";
import { t } from "../i18n";
import { navigate } from "../shell/routes";

/**
 * Motion (docs/pbi/ui/atelier-desk/README.md): entry-transition of the panel,
 * opacity and a lift of 8px in 240ms, once. It stops for prefers-reduced-motion
 * (opacity only, 120ms).
 */
const PANEL_CSS = `
@keyframes flow-sheet-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
.flow-sheet { animation: flow-sheet-in 240ms cubic-bezier(0.22, 1, 0.36, 1) 1 both; }
@keyframes flow-sheet-fade { from { opacity: 0; } to { opacity: 1; } }
@media (prefers-reduced-motion: reduce) {
  .flow-sheet { animation: flow-sheet-fade 120ms linear 1 both; }
}
`;

/** The Japanese title of a component; the id when the title is empty or a YAML block marker. */
export function componentTitle(view: ComponentView): string {
  const ja = (view.component.data.title?.ja ?? "").trim();
  if (ja === "" || ja === ">-" || ja === ">" || ja === "|" || ja === "|-") return view.component.id;
  return ja;
}

const INVOKE_KEYS: Record<string, string> = {
  skill: "invokeSkill",
  "read-skill": "invokeReadSkill",
  script: "invokeScript",
  builtin: "invokeBuiltin",
};
const WHERE_KEYS: Record<string, string> = {
  main: "whereMain",
  "subagent-readonly": "whereSubagentReadonly",
  subagent: "whereSubagent",
  runner: "whereRunner",
};
const COST_KEYS: Record<string, string> = { low: "costLow", mid: "costMid", high: "costHigh" };

/** A label of `keys[value]`, or "" when the value is not one the table knows. */
function labelOf(keys: Record<string, string>, value: string): string {
  return Object.prototype.hasOwnProperty.call(keys, value) ? t("flow", keys[value]) : "";
}

function invokeName(view: ComponentView): string {
  const inv = view.component.data.invoke;
  if (inv.kind === "skill") return inv.name;
  if (inv.kind === "read-skill") return `${inv.plugin}:${inv.path}`;
  if (inv.kind === "script") return inv.path;
  return "";
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="text-sm text-foreground">{children}</dd>
    </div>
  );
}

/** Raw value in mono next to its Japanese label. */
function Valued({ label, raw }: { label: string; raw: string }) {
  return (
    <span className="flex flex-wrap items-baseline gap-2">
      {label !== "" && <span>{label}</span>}
      <span className="font-mono text-xs text-muted-foreground">{raw}</span>
    </span>
  );
}

/** The detail panel under the graph for one component. */
export function ComponentPanel({ view }: { view: ComponentView }) {
  const data = view.component.data;
  const id = view.component.id;
  const source = view.component.source === "home" ? "home" : "bundled";
  const kindLabel = labelOf(INVOKE_KEYS, data.invoke.kind);
  const name = invokeName(view);

  return (
    <section
      aria-label={t("flow", "panelLabel")}
      className="flow-sheet flex flex-col gap-4 rounded-lg border border-foreground/10 bg-card p-5 text-card-foreground"
    >
      <style>{PANEL_CSS}</style>
      <header className="flex flex-wrap items-center gap-3">
        <h2 className="font-heading text-base text-foreground">{componentTitle(view)}</h2>
        <span
          data-source={source}
          className={`inline-flex h-6 items-center rounded-sm px-2 text-xs ${
            source === "home"
              ? "border border-dashed border-foreground/40 text-foreground"
              : "bg-muted text-muted-foreground"
          }`}
        >
          {t("flow", source === "home" ? "sourceHome" : "sourceBundled")}
        </span>
        <span className="font-mono text-xs text-muted-foreground">{id}</span>
      </header>

      <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label={t("flow", "fieldInvoke")}>
          <Valued label={kindLabel} raw={name === "" ? data.invoke.kind : `${data.invoke.kind} ${name}`} />
        </Field>
        <Field label={t("flow", "fieldWhere")}>
          <Valued label={labelOf(WHERE_KEYS, data.where)} raw={data.where} />
        </Field>
        <Field label={t("flow", "fieldCost")}>
          <Valued label={labelOf(COST_KEYS, data.cost)} raw={data.cost} />
        </Field>
        <Field label={t("flow", "fieldInteractive")}>
          {t("flow", data.interactive ? "interactiveYes" : "interactiveNo")}
        </Field>
        <Field label={t("flow", "fieldDoc")}>
          <span className="font-mono text-xs break-all">{data.doc}</span>
        </Field>
        <Field label={t("flow", "fieldWorkflows")}>
          {view.workflows.length === 0 ? (
            <span className="text-muted-foreground">{t("flow", "noWorkflows")}</span>
          ) : (
            <ul className="flex flex-wrap gap-2">
              {view.workflows.map((w) => (
                <li key={w} className="font-mono text-xs">
                  {w}
                </li>
              ))}
            </ul>
          )}
        </Field>
      </dl>

      <div className="flex flex-wrap items-center gap-3">
        {view.skillKey !== "" && (
          <button
            type="button"
            onClick={() => navigate({ screen: "skills", skillKey: view.skillKey })}
            className="inline-flex h-9 items-center rounded-md bg-primary px-4 text-sm text-primary-foreground hover:opacity-90"
          >
            {t("flow", "openSkill")}
          </button>
        )}
        {view.skillMissing && (
          <span className="inline-flex h-7 items-center rounded-sm border border-warning/60 px-2 text-xs text-foreground">
            {t("flow", "skillMissing")}
          </span>
        )}
        {view.link === "built_in" && (
          <span className="inline-flex h-7 items-center rounded-sm bg-muted px-2 text-xs text-muted-foreground">
            {t("flow", "skillBuiltIn")}
          </span>
        )}
      </div>
    </section>
  );
}
