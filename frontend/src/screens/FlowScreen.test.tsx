import { screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { ComponentView, WorkflowView } from "../api/types";
import { flow } from "../i18n/ja/flow";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import type { FakeClient } from "../test/fakeClient";
import { FlowScreen } from "./FlowScreen";

function wf(name: string, source: string, bundled: boolean, steps: [string, string[]][]): WorkflowView {
  return {
    workflow: {
      name,
      source,
      bundled,
      description: { ja: "", en: "" },
      steps: steps.map(([stage, ids]) => ({
        kind: "stage",
        stage,
        after: "",
        components: ids.map((id) => ({ id, when: "", where: "", models: [] })),
      })),
      data: {},
    },
  } as unknown as WorkflowView;
}

function comp(id: string, title: string, source: string): ComponentView {
  return {
    component: {
      id,
      source,
      data: {
        id,
        stages: [],
        title: { ja: title, en: "" },
        invoke: { kind: "skill", name: id, plugin: "", path: "", args: null },
        doc: "",
        where: "main",
        memory: "",
        when: "",
        cost: "low",
        interactive: false,
        calls_back: false,
        forbid: [],
        license: "",
        link: "",
        source,
        then: [],
      },
    },
    skillKey: "",
    link: "",
    skillMissing: false,
    workflows: [],
  } as unknown as ComponentView;
}

function clientWith(workflows: WorkflowView[], components: ComponentView[]): FakeClient {
  const client = createFakeClient();
  client.workflows.mockResolvedValue(workflows);
  client.components.mockResolvedValue(components);
  return client;
}

/** The react-flow nodes in DOM order. */
function nodes(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(".react-flow__node"));
}

beforeEach(() => {
  window.location.hash = "";
});
afterEach(() => {
  window.location.hash = "";
});

describe("FlowScreen", () => {
  it("StagesLeftToRightComponentsBelow: stage nodes follow the flow order and each stage's components come right after it", async () => {
    const client = clientWith(
      [
        wf("default", "bundled", true, [
          ["understand", ["c-read", "c-ask"]],
          ["build", ["c-impl"]],
        ]),
      ],
      [comp("c-read", "読む", "bundled"), comp("c-ask", "尋ねる", "bundled"), comp("c-impl", "実装する", "bundled")],
    );
    window.location.hash = "#/flow/default";
    const { container } = renderWithClient(<FlowScreen />, client);

    await screen.findByText("読む");
    const order = nodes(container).map((n) => n.textContent ?? "");
    const at = (needle: string): number => order.findIndex((text) => text.includes(needle));

    // Stage nodes: understand before build (left to right).
    expect(at(flow.stage_understand)).toBeGreaterThanOrEqual(0);
    expect(at(flow.stage_understand)).toBeLessThan(at(flow.stage_build));
    // Components stand with their own stage: after its head, before the next stage.
    for (const id of ["c-read", "c-ask"]) {
      expect(at(id)).toBeGreaterThan(at(flow.stage_understand));
      expect(at(id)).toBeLessThan(at(flow.stage_build));
    }
    expect(at("c-impl")).toBeGreaterThan(at(flow.stage_build));
    // Components of a stage keep the flow's order.
    expect(at("c-read")).toBeLessThan(at("c-ask"));
    // Three components and two stage heads in all.
    expect(nodes(container)).toHaveLength(5);
  });

  it("BundledAndHomeMarks: a bundled and a home flow and component carry different marks", async () => {
    const client = clientWith(
      [
        wf("alpha", "bundled", true, [["build", ["c-b", "c-h"]]]),
        wf("beta", "home", false, [["build", []]]),
        wf("gamma", "home", true, [["build", []]]),
      ],
      [comp("c-b", "同梱の部品", "bundled"), comp("c-h", "homeの部品", "home")],
    );
    window.location.hash = "#/flow/alpha";
    const { container } = renderWithClient(<FlowScreen />, client);

    await screen.findByText("同梱の部品");
    const list = screen.getByRole("navigation", { name: flow.listLabel });
    const markOf = (name: string): HTMLElement => {
      const link = within(list).getByText(name).closest("a");
      expect(link).not.toBeNull();
      const mark = (link as HTMLElement).querySelector<HTMLElement>("[data-source]");
      expect(mark).not.toBeNull();
      return mark as HTMLElement;
    };
    expect(markOf("alpha")).toHaveAttribute("data-source", "bundled");
    expect(markOf("alpha")).toHaveTextContent(flow.sourceBundled);
    expect(markOf("beta")).toHaveAttribute("data-source", "home");
    expect(markOf("beta")).toHaveTextContent(flow.sourceHome);
    expect(markOf("alpha").className).not.toBe(markOf("beta").className);
    // A home flow that has a bundled twin says it overrides it; one without does not.
    expect(within(list).getAllByText(flow.homeOverridesBundled)).toHaveLength(1);
    expect(within(list.querySelector<HTMLElement>('a[href="#/flow/gamma"]') as HTMLElement).getByText(flow.homeOverridesBundled)).toBeInTheDocument();

    // Components in the diagram.
    const compMark = (id: string): HTMLElement => {
      const node = nodes(container).find((n) => n.textContent?.includes(id));
      expect(node).toBeDefined();
      const mark = (node as HTMLElement).querySelector<HTMLElement>("[data-source]");
      expect(mark).not.toBeNull();
      return mark as HTMLElement;
    };
    expect(compMark("c-b")).toHaveAttribute("data-source", "bundled");
    expect(compMark("c-b")).toHaveTextContent(flow.sourceBundled);
    expect(compMark("c-h")).toHaveAttribute("data-source", "home");
    expect(compMark("c-h")).toHaveTextContent(flow.sourceHome);
  });
});
