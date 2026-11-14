import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { ComponentView } from "../api/types";
import { flow } from "../i18n/ja/flow";
import { FlowScreen } from "../screens/FlowScreen";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { ComponentPanel } from "./ComponentPanel";

interface Opts {
  id: string;
  title?: string;
  source?: string;
  skillKey?: string;
  skillMissing?: boolean;
  link?: string;
  workflows?: string[];
}

function view(o: Opts): ComponentView {
  return {
    component: {
      id: o.id,
      source: o.source ?? "bundled",
      data: {
        id: o.id,
        stages: ["build"],
        title: { ja: o.title ?? "", en: "" },
        invoke: { kind: "skill", name: "atelier-build", plugin: "", path: "", args: null },
        doc: "docs/build.md",
        where: "subagent",
        memory: "",
        when: "",
        cost: "high",
        interactive: true,
        calls_back: false,
        forbid: [],
        license: "",
        link: o.link ?? "",
        source: o.source ?? "bundled",
        then: [],
      },
    },
    skillKey: o.skillKey ?? "",
    link: o.link ?? "",
    skillMissing: o.skillMissing ?? false,
    workflows: o.workflows ?? [],
  } as unknown as ComponentView;
}

beforeEach(() => {
  window.location.hash = "";
});
afterEach(() => {
  window.location.hash = "";
});

describe("ComponentPanel", () => {
  it("PanelShowsDetailAndOpenSkillLink: the panel shows the title, how it is called, where, cost, interactive and the doc, and the button opens the skill", () => {
    render(
      <ComponentPanel
        view={view({ id: "build-impl", title: "実装する", skillKey: "atelier/v7", workflows: ["default"] })}
      />,
    );
    const panel = screen.getByRole("region", { name: flow.panelLabel });
    expect(within(panel).getByRole("heading", { name: "実装する" })).toBeInTheDocument();
    expect(within(panel).getByText("build-impl")).toBeInTheDocument();

    const field = (label: string): HTMLElement => {
      const dd = within(panel).getByText(label).nextElementSibling;
      expect(dd).not.toBeNull();
      return dd as HTMLElement;
    };
    expect(field(flow.fieldInvoke)).toHaveTextContent(flow.invokeSkill);
    expect(field(flow.fieldInvoke)).toHaveTextContent("atelier-build");
    expect(field(flow.fieldWhere)).toHaveTextContent(flow.whereSubagent);
    expect(field(flow.fieldCost)).toHaveTextContent(flow.costHigh);
    expect(field(flow.fieldInteractive)).toHaveTextContent(flow.interactiveYes);
    expect(field(flow.fieldDoc)).toHaveTextContent("docs/build.md");
    expect(field(flow.fieldWorkflows)).toHaveTextContent("default");

    fireEvent.click(within(panel).getByRole("button", { name: flow.openSkill }));
    expect(window.location.hash).toBe("#/skills/" + encodeURIComponent("atelier/v7"));
  });

  it("PanelShowsDetailAndOpenSkillLink: pressing a component in the diagram opens its panel", async () => {
    const workflows = [
      {
        workflow: {
          name: "default",
          source: "bundled",
          bundled: true,
          description: { ja: "", en: "" },
          steps: [
            { kind: "stage", stage: "build", after: "", components: [{ id: "build-impl", when: "", where: "", models: [] }] },
          ],
          data: {},
        },
      },
    ];
    const client = createFakeClient();
    client.workflows.mockResolvedValue(workflows as unknown as Awaited<ReturnType<typeof client.workflows>>);
    client.components.mockResolvedValue([view({ id: "build-impl", title: "実装する", skillKey: "k1" })]);
    window.location.hash = "#/flow/default";
    renderWithClient(<FlowScreen />, client);

    expect(await screen.findByTitle("実装する")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: flow.panelLabel })).not.toBeInTheDocument();
    fireEvent.click(screen.getByTitle("実装する"));

    const panel = await screen.findByRole("region", { name: flow.panelLabel });
    expect(within(panel).getByRole("heading", { name: "実装する" })).toBeInTheDocument();
    fireEvent.click(within(panel).getByRole("button", { name: flow.openSkill }));
    expect(window.location.hash).toBe("#/skills/k1");
  });

  it("SkillMissingBadge: a component whose skill is missing shows the not-installed badge", () => {
    render(<ComponentPanel view={view({ id: "ghost", title: "幽霊", skillKey: "", skillMissing: true })} />);
    const panel = screen.getByRole("region", { name: flow.panelLabel });
    expect(within(panel).getByText(flow.skillMissing)).toBeInTheDocument();
  });

  it("SkillMissingBadge: a component whose skill is present shows no such badge", () => {
    render(<ComponentPanel view={view({ id: "ok", title: "在る", skillKey: "k", skillMissing: false })} />);
    expect(screen.queryByText(flow.skillMissing)).not.toBeInTheDocument();
  });
});
