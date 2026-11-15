import { createElement } from "react";
import type { ReactElement } from "react";
import { render } from "@testing-library/react";
import type { RenderResult } from "@testing-library/react";
import { vi } from "vitest";
import type { Mock } from "vitest";
import { ClientProvider } from "../api/client";
import type { DeskClient } from "../api/client";
import type {
  CasesView,
  ComponentView,
  Connection,
  HomeView,
  RunsView,
  SaveResult,
  SkillView,
  WorkflowView,
} from "../api/types";

/** A DeskClient whose every method is a vi.fn (events included). */
export type FakeClient = {
  [K in keyof DeskClient]: DeskClient[K] extends (...args: infer A) => infer R
    ? Mock<(...args: A) => R>
    : { [E in keyof DeskClient[K]]: DeskClient[K][E] extends (...args: infer A) => infer R ? Mock<(...args: A) => R> : never };
};

const emptyConnection = {
  state: "ready",
  version: "",
  python: "",
  pythonVersion: "",
  home: "",
  code: "",
  diagnostic: "",
  saving: false,
} as unknown as Connection;

/**
 * Builds a typed fake DeskClient. Every method is a vi.fn that resolves an
 * empty, plausible value; `overrides` replaces any of them. `events`
 * subscriptions return a no-op unsubscribe function.
 */
export function createFakeClient(overrides: Partial<DeskClient> = {}): FakeClient {
  const base = {
    connection: vi.fn(() => Promise.resolve(emptyConnection)),
    versions: vi.fn(() => Promise.resolve([] as string[])),
    switchVersion: vi.fn((_v: string) => Promise.resolve(emptyConnection)),
    setPythonPath: vi.fn((_p: string) => Promise.resolve(emptyConnection)),
    recheck: vi.fn(() => Promise.resolve(emptyConnection)),
    workflows: vi.fn(() => Promise.resolve([] as WorkflowView[])),
    workflow: vi.fn((_name: string) => Promise.resolve({} as unknown as WorkflowView)),
    components: vi.fn(() => Promise.resolve([] as ComponentView[])),
    component: vi.fn((_id: string) => Promise.resolve({} as unknown as ComponentView)),
    skills: vi.fn(() => Promise.resolve([])),
    skill: vi.fn((_key: string) => Promise.resolve({} as unknown as SkillView)),
    skillFile: vi.fn((_key: string, _rel: string) => Promise.resolve("")),
    homeState: vi.fn(() => Promise.resolve({} as unknown as HomeView)),
    saveSetting: vi.fn((_req: Parameters<DeskClient["saveSetting"]>[0]) =>
      Promise.resolve({} as unknown as SaveResult),
    ),
    repos: vi.fn(() => Promise.resolve([] as string[])),
    addRepo: vi.fn((_dir: string) => Promise.resolve([] as string[])),
    removeRepo: vi.fn((_dir: string) => Promise.resolve([] as string[])),
    runs: vi.fn(() => Promise.resolve({ runs: [], warnings: [] } as unknown as RunsView)),
    cases: vi.fn(() => Promise.resolve({ cases: [], warnings: [] } as unknown as CasesView)),
    events: {
      onHomeChanged: vi.fn((_cb: Parameters<DeskClient["events"]["onHomeChanged"]>[0]) => () => {}),
      onJobOutput: vi.fn((_cb: Parameters<DeskClient["events"]["onJobOutput"]>[0]) => () => {}),
      onConnectionChanged: vi.fn(
        (_cb: Parameters<DeskClient["events"]["onConnectionChanged"]>[0]) => () => {},
      ),
    },
  } satisfies Record<keyof DeskClient, unknown>;
  return { ...base, ...overrides } as unknown as FakeClient;
}

/** Renders `ui` inside a ClientProvider; returns render's result and the client used. */
export function renderWithClient(
  ui: ReactElement,
  client: FakeClient = createFakeClient(),
): RenderResult & { client: FakeClient } {
  const result = render(createElement(ClientProvider, { client: client as unknown as DeskClient }, ui));
  return { ...result, client };
}
