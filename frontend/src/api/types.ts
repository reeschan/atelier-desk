// Types the screen uses. They are narrowed aliases of the generated
// wailsjs/go/models classes: no field name is written by hand, the generated
// classes stay out of the screen, and the Go-side json.RawMessage / any fields
// are put back as `unknown`.
import type {
  atelier,
  catalog,
  desk,
  homestore,
  runs,
  skills,
} from "../../wailsjs/go/models";

/** Plain data of a generated class: methods (convertValues, ...) are dropped. */
export type Plain<T> = T extends (infer U)[]
  ? Plain<U>[]
  : T extends object
    ? { [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]: Plain<T[K]> }
    : T;

/** Plain data of T with the fields named in O replaced by O's types. */
export type Override<T, O> = Omit<Plain<T>, keyof O> & O;

// --- connection ---

export const ConnState = {
  Checking: "checking",
  Ready: "ready",
  PythonUnavailable: "python_unavailable",
  Unsupported: "unsupported",
} as const;
export type ConnState = (typeof ConnState)[keyof typeof ConnState];

export type Connection = Override<desk.Connection, { state: ConnState }>;

// --- components and workflows ---

export type Component = Plain<atelier.Component>;
export type Usage = Plain<catalog.Usage>;
export type ComponentView = Plain<catalog.ComponentView>;

export type Workflow = Override<atelier.Workflow, { data: unknown }>;
export type WorkflowView = Override<catalog.WorkflowView, { workflow: Workflow }>;

// --- skills ---

export type Skill = Plain<skills.Skill>;
export type VersionInfo = Plain<skills.VersionInfo>;
export type SkillDetail = Plain<skills.SkillDetail>;
export type SkillView = Plain<desk.SkillView>;

// --- home settings ---

export type HomeState = Plain<atelier.State>;
export type Baseline = Plain<homestore.Baseline>;
export type HomeView = Plain<desk.HomeView>;

export const Outcome = {
  Saved: "saved",
  Rejected: "rejected",
  ExternalChange: "external_change",
  PartialFailure: "partial_failure",
} as const;
export type Outcome = (typeof Outcome)[keyof typeof Outcome];

/** kind, name, input and base (no overwrite flag). */
export type SaveSettingRequest = Plain<desk.SaveSettingRequest>;

/** Carries current and base so the screen can go on from the new baseline. */
export type SaveResult = Override<homestore.SaveResult, { Outcome: Outcome }>;

// --- runs and cases ---

export type ReadWarning = Plain<runs.ReadWarning>;
export type Attempt = Override<
  runs.Attempt,
  { Raw: unknown; Findings: unknown; Verify: unknown }
>;
export type PBIResult = Override<runs.PBIResult, { Attempts: Attempt[] }>;
export type Run = Override<runs.Run, { PBIs: PBIResult[] }>;
export type Case = Override<runs.Case, { Raw: unknown }>;
export type RunsView = Override<desk.RunsView, { runs: Run[] }>;
export type CasesView = Override<desk.CasesView, { cases: Case[] }>;
