import { createContext, createElement, useContext } from "react";
import type { ReactNode } from "react";
import * as Service from "../../wailsjs/go/desk/Service";
import { createWailsEvents } from "./events";
import type { DeskEvents } from "./events";
import type {
  CasesView,
  ComponentView,
  Connection,
  HomeView,
  RunsView,
  SaveResult,
  SaveSettingRequest,
  Skill,
  SkillView,
  WorkflowView,
} from "./types";

export interface DeskClient {
  connection(): Promise<Connection>;
  versions(): Promise<string[]>;
  switchVersion(v: string): Promise<Connection>;
  setPythonPath(p: string): Promise<Connection>;
  recheck(): Promise<Connection>;
  workflows(): Promise<WorkflowView[]>;
  workflow(name: string): Promise<WorkflowView>;
  components(): Promise<ComponentView[]>;
  component(id: string): Promise<ComponentView>;
  skills(): Promise<Skill[]>;
  skill(key: string): Promise<SkillView>;
  skillFile(key: string, rel: string): Promise<string>;
  homeState(): Promise<HomeView>;
  saveSetting(req: SaveSettingRequest): Promise<SaveResult>;
  repos(): Promise<string[]>;
  addRepo(dir: string): Promise<string[]>;
  removeRepo(dir: string): Promise<string[]>;
  runs(): Promise<RunsView>;
  cases(): Promise<CasesView>;
  events: DeskEvents;
}

/** An error from Go: code is what the screen turns into a text, detail is extra. */
export class DeskError extends Error {
  code: string;
  detail: string;

  constructor(code: string, detail: string) {
    super(detail === "" ? code : `${code}: ${detail}`);
    this.name = "DeskError";
    this.code = code;
    this.detail = detail;
  }
}

function parseGoError(text: string): DeskError | null {
  let v: unknown;
  try {
    v = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof v !== "object" || v === null) return null;
  const o = v as Record<string, unknown>;
  if (typeof o.code !== "string" || o.code === "") return null;
  return new DeskError(o.code, typeof o.detail === "string" ? o.detail : "");
}

/**
 * Turns anything a rejected call carries into a DeskError. Go's error text is
 * `{"code","detail"}` JSON; anything else becomes code `unknown` with the
 * original text in detail.
 */
export function toDeskError(e: unknown): DeskError {
  if (e instanceof DeskError) return e;
  const text = typeof e === "string" ? e : e instanceof Error ? e.message : String(e);
  return parseGoError(text) ?? new DeskError("unknown", text);
}

/** Awaits a generated call; a reject becomes a DeskError, the value is typed for the screen. */
async function call<T>(p: Promise<unknown>): Promise<T> {
  try {
    return (await p) as T;
  } catch (e) {
    throw toDeskError(e);
  }
}

export function createWailsClient(): DeskClient {
  return {
    connection: () => call(Service.Connection()),
    versions: () => call(Service.Versions()),
    switchVersion: (v) => call(Service.SwitchVersion(v)),
    setPythonPath: (p) => call(Service.SetPythonPath(p)),
    recheck: () => call(Service.Recheck()),
    workflows: () => call(Service.Workflows()),
    workflow: (name) => call(Service.Workflow(name)),
    components: () => call(Service.Components()),
    component: (id) => call(Service.Component(id)),
    skills: () => call(Service.Skills()),
    skill: (key) => call(Service.Skill(key)),
    skillFile: (key, rel) => call(Service.SkillFile(key, rel)),
    homeState: () => call(Service.HomeState()),
    saveSetting: (req) =>
      call(Service.SaveSetting(req as Parameters<typeof Service.SaveSetting>[0])),
    repos: () => call(Service.Repos()),
    addRepo: (dir) => call(Service.AddRepo(dir)),
    removeRepo: (dir) => call(Service.RemoveRepo(dir)),
    runs: () => call(Service.Runs()),
    cases: () => call(Service.Cases()),
    events: createWailsEvents(),
  };
}

export const ClientContext = createContext<DeskClient | null>(null);

export function ClientProvider({
  client,
  children,
}: {
  client: DeskClient;
  children?: ReactNode;
}) {
  return createElement(ClientContext.Provider, { value: client }, children);
}

export function useClient(): DeskClient {
  const client = useContext(ClientContext);
  if (client === null) {
    throw new Error("useClient: no ClientProvider above this component");
  }
  return client;
}
