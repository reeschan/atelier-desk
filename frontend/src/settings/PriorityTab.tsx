import { useRef, useState } from "react";
import { DeskError, useClient } from "../api/client";
import type { HomeState, SaveResult } from "../api/types";
import { t } from "../i18n";
import { SaveBar } from "./SaveBar";
import { useHomeSave } from "./useHomeSave";
import type { TabProps } from "./useHomeSave";

const TIERS = ["haiku", "sonnet", "opus"] as const;
type Tier = (typeof TIERS)[number];

type TemplateState = HomeState["templates"][string];
type TemplateStepState = NonNullable<TemplateState["tiers"]>[string][number];

type StepEdit = { runner: string; model: string; effort: string; until: string };
type Item = { kind: "step"; step: StepEdit } | { kind: "rotate"; members: StepEdit[] };
type TierEdit = { on: boolean; items: Item[] };
type TplForm = { name: string; description: string; tiers: Record<Tier, TierEdit> };

type Move = -1 | 1;

const FIELD = "h-8 rounded-md border border-foreground/10 bg-background px-2 text-xs";
const SMALL =
  "desk-tint h-8 rounded-md border border-foreground/15 px-2 text-xs hover:bg-muted disabled:opacity-40";
const CARD = "rounded-lg border border-foreground/10 bg-card p-5";

const EMPTY_STEP: StepEdit = { runner: "", model: "", effort: "", until: "" };

function toStep(s: { runner?: string; model?: string; effort?: string | null; until?: string | null }): StepEdit {
  return { runner: s.runner ?? "", model: s.model ?? "", effort: s.effort ?? "", until: s.until ?? "" };
}

function toItem(s: TemplateStepState): Item {
  if (s.rotate !== undefined && s.rotate !== null) return { kind: "rotate", members: s.rotate.map(toStep) };
  return { kind: "step", step: toStep(s) };
}

function emptyForm(): TplForm {
  const tiers = {} as Record<Tier, TierEdit>;
  for (const tier of TIERS) tiers[tier] = { on: false, items: [] };
  return { name: "", description: "", tiers };
}

function fromTemplate(tpl: TemplateState): TplForm {
  const form = emptyForm();
  form.name = tpl.name;
  form.description = tpl.description ?? "";
  for (const tier of TIERS) {
    const steps = tpl.tiers?.[tier];
    if (steps !== undefined && steps !== null) form.tiers[tier] = { on: true, items: steps.map(toItem) };
  }
  return form;
}

function stepBody(s: StepEdit): Record<string, unknown> {
  const body: Record<string, unknown> = { runner: s.runner, model: s.model };
  if (s.effort !== "") body.effort = s.effort;
  if (s.until !== "") body.until = s.until;
  return body;
}

/** The body for `template --name <name>`: name, description and the tiers that are on. */
function serializeTemplate(f: TplForm): string {
  const tiers: Record<string, unknown[]> = {};
  for (const tier of TIERS) {
    const { on, items } = f.tiers[tier];
    if (!on) continue;
    tiers[tier] = items.map((it) =>
      it.kind === "step" ? stepBody(it.step) : { rotate: it.members.map(stepBody) },
    );
  }
  return JSON.stringify({ name: f.name, description: f.description, tiers });
}

function moveIn<T>(list: T[], index: number, delta: Move): T[] {
  const to = index + delta;
  if (to < 0 || to >= list.length) return list;
  const out = [...list];
  const [moved] = out.splice(index, 1);
  out.splice(to, 0, moved);
  return out;
}

// --- one step's fields ---

function StepFields({
  label,
  step,
  runnerList,
  onChange,
}: {
  label: string;
  step: StepEdit;
  runnerList: string;
  onChange: (s: StepEdit) => void;
}) {
  return (
    <>
      <input
        type="text"
        list={runnerList}
        className={`${FIELD} w-28 font-mono`}
        value={step.runner}
        placeholder={t("settings2", "fieldRunner")}
        aria-label={`${label} ${t("settings2", "fieldRunner")}`}
        onChange={(e) => onChange({ ...step, runner: e.target.value })}
      />
      <input
        type="text"
        className={`${FIELD} w-56 font-mono`}
        value={step.model}
        placeholder={t("settings2", "fieldModel")}
        aria-label={`${label} ${t("settings2", "fieldModel")}`}
        onChange={(e) => onChange({ ...step, model: e.target.value })}
      />
      <input
        type="text"
        className={`${FIELD} w-24 font-mono`}
        value={step.effort}
        placeholder={t("settings2", "fieldEffort")}
        aria-label={`${label} ${t("settings2", "fieldEffort")}`}
        onChange={(e) => onChange({ ...step, effort: e.target.value })}
      />
      <input
        type="date"
        className={`${FIELD} font-mono`}
        value={step.until}
        aria-label={`${label} ${t("settings2", "fieldUntil")}`}
        onChange={(e) => onChange({ ...step, until: e.target.value })}
      />
    </>
  );
}

function MoveButtons({
  label,
  index,
  count,
  onMove,
  onRemove,
}: {
  label: string;
  index: number;
  count: number;
  onMove: (d: Move) => void;
  onRemove: () => void;
}) {
  return (
    <>
      <button
        type="button"
        className={SMALL}
        disabled={index === 0}
        aria-label={`${label} ${t("settings2", "itemUp")}`}
        onClick={() => onMove(-1)}
      >
        ↑
      </button>
      <button
        type="button"
        className={SMALL}
        disabled={index === count - 1}
        aria-label={`${label} ${t("settings2", "itemDown")}`}
        onClick={() => onMove(1)}
      >
        ↓
      </button>
      <button
        type="button"
        className={SMALL}
        aria-label={`${label} ${t("settings2", "itemRemove")}`}
        onClick={onRemove}
      >
        ×
      </button>
    </>
  );
}

// --- one template's form ---

type EditorValue = { form: TplForm; runners: string[] };

/**
 * The form of one template (origin "" = a new one). A rename saves the new name and then
 * deletes the old one; both go through the same useHomeSave.
 */
function TemplateEditor({
  origin,
  ready,
  onSaved,
}: {
  origin: string;
  ready: boolean;
  onSaved: (name: string) => void;
}) {
  const client = useClient();
  const h = useHomeSave<EditorValue>({
    kind: "template",
    ready,
    load: async () => {
      const v = await client.homeState();
      const runners = Object.keys(v.state.runners).sort();
      if (origin === "") return { value: { form: emptyForm(), runners }, base: v.base };
      const tpl = v.state.templates[origin];
      if (tpl === undefined) throw new DeskError("not_found", origin);
      return { value: { form: fromTemplate(tpl), runners }, base: v.base };
    },
    serialize: (v) => serializeTemplate(v.form),
    watches: (paths) => paths.some((p) => p.startsWith("templates/")),
  });
  // True once the second step (delete the old name) has been sent: an overwrite then re-sends
  // that delete, so finish() must not send it a second time.
  const deleteSent = useRef(false);
  const d = h.draft;
  if (d === null) return null;
  const form = d.form;
  const listId = `runners-${origin === "" ? "new" : origin}`;

  function editForm(fn: (f: TplForm) => TplForm) {
    h.setDraft((prev) => ({ ...prev, form: fn(prev.form) }));
  }
  function editTier(tier: Tier, fn: (x: TierEdit) => TierEdit) {
    editForm((f) => ({ ...f, tiers: { ...f.tiers, [tier]: fn(f.tiers[tier]) } }));
  }
  function editItems(tier: Tier, fn: (items: Item[]) => Item[]) {
    editTier(tier, (x) => ({ ...x, items: fn(x.items) }));
  }

  async function finish(r: SaveResult | null): Promise<SaveResult | null> {
    if (r === null || r.Outcome !== "saved") return r;
    let out: SaveResult | null = r;
    if (origin !== "" && form.name !== origin && !deleteSent.current) {
      deleteSent.current = true;
      out = await h.save({ kind: "template-delete", name: origin, input: "{}" });
      if (out === null || out.Outcome !== "saved") return out;
    }
    deleteSent.current = false;
    onSaved(form.name);
    return out;
  }

  const bar = {
    ...h,
    save: async () => {
      deleteSent.current = false;
      return finish(await h.save({ name: form.name }));
    },
    overwrite: async () => finish(await h.overwrite()),
  };

  const renaming = origin !== "" && form.name !== origin;

  return (
    <section
      aria-label={origin === "" ? t("settings2", "editorNewTitle") : t("settings2", "editorEditTitle", { name: origin })}
      data-template-editor={origin}
      className={CARD}
    >
      <h3 className="font-heading text-lg">
        {origin === "" ? t("settings2", "editorNewTitle") : t("settings2", "editorEditTitle", { name: origin })}
      </h3>
      <datalist id={listId}>
        {d.runners.map((r) => (
          <option key={r} value={r} />
        ))}
      </datalist>
      <div className="mt-3 flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "fieldName")}
          <input
            type="text"
            className={`${FIELD} w-72 font-mono`}
            value={form.name}
            aria-label={t("settings2", "fieldName")}
            onChange={(e) => editForm((f) => ({ ...f, name: e.target.value }))}
          />
          <span className="text-muted-foreground">{t("settings2", "nameHint")}</span>
          {renaming && <span className="text-muted-foreground">{t("settings2", "renameNote")}</span>}
        </label>
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "fieldDescription")}
          <input
            type="text"
            className={`${FIELD} w-full`}
            value={form.description}
            aria-label={t("settings2", "fieldDescription")}
            onChange={(e) => editForm((f) => ({ ...f, description: e.target.value }))}
          />
        </label>
        {TIERS.map((tier) => {
          const { on, items } = form.tiers[tier];
          return (
            <div key={tier} data-tier={tier} className="rounded-md border border-foreground/10 p-3">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={on}
                  aria-label={t("settings2", "tierOn", { tier })}
                  onChange={(e) => editTier(tier, (x) => ({ ...x, on: e.target.checked }))}
                />
                {t("settings2", "tierOn", { tier })}
              </label>
              {on && (
                <>
                  {items.length === 0 && (
                    <p className="mt-2 text-xs text-muted-foreground">{t("settings2", "tierEmpty")}</p>
                  )}
                  <ol className="mt-2 flex flex-col gap-2">
                    {items.map((it, i) => {
                      const label = `${tier} ${t("settings2", "stepLabel", { n: i + 1 })}`;
                      const move = (delta: Move) => editItems(tier, (list) => moveIn(list, i, delta));
                      const remove = () => editItems(tier, (list) => list.filter((_, j) => j !== i));
                      if (it.kind === "step") {
                        return (
                          <li key={i} aria-label={label} className="flex flex-wrap items-center gap-2">
                            <span className="w-6 font-mono text-xs text-muted-foreground">{i + 1}</span>
                            <StepFields
                              label={label}
                              step={it.step}
                              runnerList={listId}
                              onChange={(s) =>
                                editItems(tier, (list) =>
                                  list.map((x, j) => (j === i ? { kind: "step", step: s } : x)),
                                )
                              }
                            />
                            <MoveButtons label={label} index={i} count={items.length} onMove={move} onRemove={remove} />
                          </li>
                        );
                      }
                      return (
                        <li
                          key={i}
                          aria-label={label}
                          data-rotate
                          className="flex flex-col gap-2 rounded-md border border-foreground/10 p-2"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="w-6 font-mono text-xs text-muted-foreground">{i + 1}</span>
                            <span className="text-xs">{t("settings2", "rotateTitle")}</span>
                            <span className="text-xs text-muted-foreground">{t("settings2", "rotateNote")}</span>
                            <MoveButtons label={label} index={i} count={items.length} onMove={move} onRemove={remove} />
                          </div>
                          <ol className="flex flex-col gap-2 pl-6">
                            {it.members.map((m, k) => {
                              const mlabel = `${label} ${t("settings2", "memberLabel", { n: k + 1 })}`;
                              const setMembers = (fn: (ms: StepEdit[]) => StepEdit[]) =>
                                editItems(tier, (list) =>
                                  list.map((x, j) =>
                                    j === i && x.kind === "rotate" ? { kind: "rotate", members: fn(x.members) } : x,
                                  ),
                                );
                              return (
                                <li key={k} aria-label={mlabel} className="flex flex-wrap items-center gap-2">
                                  <StepFields
                                    label={mlabel}
                                    step={m}
                                    runnerList={listId}
                                    onChange={(s) => setMembers((ms) => ms.map((x, n) => (n === k ? s : x)))}
                                  />
                                  <MoveButtons
                                    label={mlabel}
                                    index={k}
                                    count={it.members.length}
                                    onMove={(delta) => setMembers((ms) => moveIn(ms, k, delta))}
                                    onRemove={() => setMembers((ms) => ms.filter((_, n) => n !== k))}
                                  />
                                </li>
                              );
                            })}
                          </ol>
                          <div className="pl-6">
                            <button
                              type="button"
                              className={SMALL}
                              onClick={() =>
                                editItems(tier, (list) =>
                                  list.map((x, j) =>
                                    j === i && x.kind === "rotate"
                                      ? { kind: "rotate", members: [...x.members, { ...EMPTY_STEP }] }
                                      : x,
                                  ),
                                )
                              }
                            >
                              {t("settings2", "addMember")}
                            </button>
                          </div>
                        </li>
                      );
                    })}
                  </ol>
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      className={SMALL}
                      onClick={() => editItems(tier, (list) => [...list, { kind: "step", step: { ...EMPTY_STEP } }])}
                    >
                      {t("settings2", "addStep")}
                    </button>
                    <button
                      type="button"
                      className={SMALL}
                      onClick={() =>
                        editItems(tier, (list) => [
                          ...list,
                          { kind: "rotate", members: [{ ...EMPTY_STEP }, { ...EMPTY_STEP }] },
                        ])
                      }
                    >
                      {t("settings2", "addRotate")}
                    </button>
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
      <SaveBar save={bar} ready={ready} />
    </section>
  );
}

// --- application ---

type AppForm = { template: string; until: string; onExpire: "ask" | "drop"; reason: string };
type AppValue = {
  form: AppForm;
  templates: string[];
  status: string;
  current: { template: string; until: string; reason: string } | null;
};

/** "2026-11-13T14:59" (datetime-local) as an ISO time with the local offset. */
function withOffset(local: string): string {
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return local;
  const off = -d.getTimezoneOffset();
  const sign = off < 0 ? "-" : "+";
  const abs = Math.abs(off);
  const hh = String(Math.floor(abs / 60)).padStart(2, "0");
  const mm = String(abs % 60).padStart(2, "0");
  const base = local.length === 16 ? `${local}:00` : local;
  return `${base}${sign}${hh}:${mm}`;
}

function serializeApp(v: AppValue): string {
  const f = v.form;
  const body: Record<string, unknown> = {
    template: f.template,
    until: f.until === "" ? null : withOffset(f.until),
    on_expire: f.onExpire,
  };
  if (f.reason !== "") body.reason = f.reason;
  return JSON.stringify(body);
}

const STATUS_TEXT: Record<string, string> = {
  none: "appStatusNone",
  active: "appStatusActive",
  expired_ask: "appStatusExpiredAsk",
  expired_drop: "appStatusExpiredDrop",
  unknown_template: "appStatusUnknownTemplate",
};

function fromAppState(s: HomeState): AppValue {
  const templates = Object.keys(s.templates).sort();
  const cur = s.application;
  return {
    form: {
      template: cur?.template ?? templates[0] ?? "",
      until: "",
      onExpire: "ask",
      reason: "",
    },
    templates,
    status: s.application_status,
    current:
      cur === undefined || cur === null
        ? null
        : { template: cur.template, until: cur.until ?? "", reason: cur.reason ?? "" },
  };
}

function ApplicationPanel({ ready }: { ready: boolean }) {
  const client = useClient();
  const [extend, setExtend] = useState("");
  const h = useHomeSave<AppValue>({
    kind: "application",
    ready,
    load: async () => {
      const v = await client.homeState();
      return { value: fromAppState(v.state), base: v.base };
    },
    serialize: serializeApp,
    watches: (paths) =>
      paths.some((p) => p === "overrides.json" || p === "history.jsonl" || p.startsWith("templates/")),
  });
  const v = h.value;
  const d = h.draft;
  if (v === null || d === null) return null;

  function setForm(fn: (f: AppForm) => AppForm) {
    h.setDraft((prev) => ({ ...prev, form: fn(prev.form) }));
  }

  function answer(choice: "extend" | "stop" | "forever") {
    const input: { choice: string; until?: string } = { choice };
    if (choice === "extend") input.until = withOffset(extend);
    void h.save({ kind: "application-answer", name: "", input: JSON.stringify(input) });
  }

  const statusKey = STATUS_TEXT[v.status] ?? "appStatusNone";
  return (
    <section aria-label={t("settings2", "appTitle")} data-application-status={v.status} className={CARD}>
      <h3 className="font-heading text-lg">{t("settings2", "appTitle")}</h3>
      <p className="mt-2 text-sm" role="status">
        {t("settings2", "appStatusLabel")}: {t("settings2", statusKey)}
      </p>
      {v.current !== null && (
        <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
          <dt className="text-muted-foreground">{t("settings2", "appCurrentTemplate")}</dt>
          <dd className="font-mono">{v.current.template}</dd>
          <dt className="text-muted-foreground">{t("settings2", "appCurrentUntil")}</dt>
          <dd className="font-mono">{v.current.until === "" ? t("settings2", "appNoLimit") : v.current.until}</dd>
          {v.current.reason !== "" && (
            <>
              <dt className="text-muted-foreground">{t("settings2", "appCurrentReason")}</dt>
              <dd>{v.current.reason}</dd>
            </>
          )}
        </dl>
      )}
      {v.status === "expired_ask" && (
        <div className="mt-3 rounded-md border border-warning/50 p-3" data-answer>
          <p className="text-sm">{t("settings2", "answerTitle")}</p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <label className="flex items-center gap-2 text-xs">
              {t("settings2", "answerUntil")}
              <input
                type="datetime-local"
                className={`${FIELD} font-mono`}
                value={extend}
                aria-label={t("settings2", "answerUntil")}
                onChange={(e) => setExtend(e.target.value)}
              />
            </label>
            <button
              type="button"
              className={SMALL}
              disabled={!ready || extend === ""}
              onClick={() => answer("extend")}
            >
              {t("settings2", "answerExtend")}
            </button>
            <button type="button" className={SMALL} disabled={!ready} onClick={() => answer("stop")}>
              {t("settings2", "answerStop")}
            </button>
            <button type="button" className={SMALL} disabled={!ready} onClick={() => answer("forever")}>
              {t("settings2", "answerForever")}
            </button>
          </div>
          {extend === "" && <p className="mt-1 text-xs text-muted-foreground">{t("settings2", "answerNeedUntil")}</p>}
        </div>
      )}
      <h4 className="mt-4 text-sm">{t("settings2", "appStartTitle")}</h4>
      <div className="mt-2 flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "appTemplate")}
          <select
            className={`${FIELD} w-72 font-mono`}
            value={d.form.template}
            aria-label={t("settings2", "appTemplate")}
            onChange={(e) => setForm((f) => ({ ...f, template: e.target.value }))}
          >
            {d.templates.length === 0 && <option value="">{t("settings2", "appTemplateNone")}</option>}
            {d.templates.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "appUntil")}
          <input
            type="datetime-local"
            className={`${FIELD} w-64 font-mono`}
            value={d.form.until}
            aria-label={t("settings2", "appUntil")}
            onChange={(e) => setForm((f) => ({ ...f, until: e.target.value }))}
          />
          <span className="text-muted-foreground">{t("settings2", "appUntilHint")}</span>
        </label>
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "appOnExpire")}
          <select
            className={`${FIELD} w-64`}
            value={d.form.onExpire}
            aria-label={t("settings2", "appOnExpire")}
            onChange={(e) => setForm((f) => ({ ...f, onExpire: e.target.value === "drop" ? "drop" : "ask" }))}
          >
            <option value="ask">{t("settings2", "appOnExpireAsk")}</option>
            <option value="drop">{t("settings2", "appOnExpireDrop")}</option>
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs">
          {t("settings2", "appReason")}
          <input
            type="text"
            className={`${FIELD} w-full`}
            value={d.form.reason}
            aria-label={t("settings2", "appReason")}
            onChange={(e) => setForm((f) => ({ ...f, reason: e.target.value }))}
          />
        </label>
      </div>
      <SaveBar save={h} ready={ready} />
    </section>
  );
}

// --- the tab ---

type ListValue = { templates: string[]; examples: string[] };

/** Priority templates: make, edit, delete, copy from an example, apply, answer an expiry (user stories 22, 23). */
export function PriorityTab({ ready }: TabProps) {
  const client = useClient();
  const [selected, setSelected] = useState<string | null>(null);
  const h = useHomeSave<ListValue>({
    kind: "template-delete",
    ready,
    load: async () => {
      const v = await client.homeState();
      return {
        value: {
          templates: Object.keys(v.state.templates).sort(),
          examples: Object.keys(v.state.examples).sort(),
        },
        base: v.base,
      };
    },
    serialize: (v) => JSON.stringify(v),
    watches: (paths) => paths.some((p) => p.startsWith("templates/")),
  });
  const v = h.value;

  async function remove(name: string) {
    const r = await h.save({ kind: "template-delete", name, input: "{}" });
    if (r !== null && r.Outcome === "saved") setSelected((cur) => (cur === name ? null : cur));
  }

  async function copy(example: string) {
    const r = await h.save({ kind: "example-copy", name: example, input: "{}" });
    if (r !== null && r.Outcome === "saved") setSelected(example);
  }

  if (v === null) return null;
  const busy = !ready || (h.state !== "viewing" && h.state !== "editing" && h.state !== "rejected");

  return (
    <div className="flex flex-col gap-4">
      <section aria-label={t("settings2", "priorityListTitle")} className={CARD}>
        <div className="flex items-center justify-between gap-3">
          <h3 className="font-heading text-lg">{t("settings2", "priorityListTitle")}</h3>
          <button type="button" className={SMALL} onClick={() => setSelected("")}>
            {t("settings2", "tplNew")}
          </button>
        </div>
        {v.templates.length === 0 && (
          <p className="mt-2 text-xs text-muted-foreground">{t("settings2", "priorityListEmpty")}</p>
        )}
        <ul className="mt-3 flex flex-col gap-2">
          {v.templates.map((n) => (
            <li key={n} data-template={n} className="flex items-center gap-2">
              <span className="font-mono text-sm">{n}</span>
              <button
                type="button"
                className={SMALL}
                aria-label={`${n} ${t("settings2", "tplEdit")}`}
                onClick={() => setSelected(n)}
              >
                {t("settings2", "tplEdit")}
              </button>
              <button
                type="button"
                className={SMALL}
                disabled={busy}
                aria-label={`${n} ${t("settings2", "tplDelete")}`}
                onClick={() => void remove(n)}
              >
                {t("settings2", "tplDelete")}
              </button>
            </li>
          ))}
        </ul>
        <p className="mt-2 text-xs text-muted-foreground">{t("settings2", "tplDeleteNote")}</p>
        <h4 className="mt-4 text-sm">{t("settings2", "examplesTitle")}</h4>
        {v.examples.length === 0 && (
          <p className="mt-1 text-xs text-muted-foreground">{t("settings2", "examplesEmpty")}</p>
        )}
        <ul className="mt-2 flex flex-col gap-2">
          {v.examples.map((n) => (
            <li key={n} data-example={n} className="flex items-center gap-2">
              <span className="font-mono text-sm">{n}</span>
              <button
                type="button"
                className={SMALL}
                disabled={busy}
                aria-label={`${n} ${t("settings2", "exampleCopy")}`}
                onClick={() => void copy(n)}
              >
                {t("settings2", "exampleCopy")}
              </button>
            </li>
          ))}
        </ul>
        <p className="mt-2 text-xs text-muted-foreground">{t("settings2", "exampleCopyNote")}</p>
        <SaveBar save={h} ready={ready} />
      </section>
      {selected !== null && (
        <TemplateEditor
          key={selected}
          origin={selected}
          ready={ready}
          onSaved={(name) => {
            setSelected(name);
            void h.reload();
          }}
        />
      )}
      <ApplicationPanel ready={ready} />
    </div>
  );
}
