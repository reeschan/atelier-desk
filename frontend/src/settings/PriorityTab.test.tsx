import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { HomeState, HomeView, SaveResult } from "../api/types";
import { t } from "../i18n";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import type { FakeClient } from "../test/fakeClient";
import { PriorityTab } from "./PriorityTab";

const BASE = { Taken: null, Files: { "templates/mine.json": "aaa" } };

type Overrides = {
  templates?: Record<string, unknown>;
  examples?: Record<string, unknown>;
  status?: string;
  application?: unknown;
};

function view(o: Overrides = {}): HomeView {
  return {
    state: {
      home: "",
      runners: { claude: {} },
      templates: o.templates ?? {},
      examples: o.examples ?? {},
      application: o.application,
      application_status: o.status ?? "none",
      warnings: [],
    },
    base: BASE,
  } as unknown as HomeView;
}

function result(r: Partial<SaveResult>): SaveResult {
  return { Base: BASE, Current: BASE, Problems: [], ...r } as unknown as SaveResult;
}

function setup(o: Overrides, saved: SaveResult = result({ Outcome: "saved" })): FakeClient {
  const client = createFakeClient();
  client.homeState.mockResolvedValue(view(o));
  client.saveSetting.mockResolvedValue(saved);
  renderWithClient(<PriorityTab state={{} as HomeState} base={BASE} ready={true} />, client);
  return client;
}

function lastRequest(client: FakeClient) {
  const calls = client.saveSetting.mock.calls;
  return calls[calls.length - 1][0];
}

describe("PriorityTab", () => {
  it("TemplateCreateDeleteExampleCopy", async () => {
    const client = setup({
      templates: { mine: { name: "mine", description: "d", tiers: {} } },
      examples: { sample: { name: "sample", description: "", tiers: {} } },
    });

    // create
    fireEvent.click(await screen.findByRole("button", { name: t("settings2", "tplNew") }));
    const editor = await screen.findByRole("region", { name: t("settings2", "editorNewTitle") });
    fireEvent.change(within(editor).getByRole("textbox", { name: t("settings2", "fieldName") }), {
      target: { value: "fresh" },
    });
    fireEvent.click(within(editor).getByRole("button", { name: t("settings", "save") }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    const created = lastRequest(client);
    expect(created.kind).toBe("template");
    expect(created.name).toBe("fresh");
    const body = JSON.parse(created.input) as Record<string, unknown>;
    expect(body.name).toBe("fresh");
    expect(body).toHaveProperty("description");
    expect(body).toHaveProperty("tiers");

    // delete
    fireEvent.click(await screen.findByRole("button", { name: `mine ${t("settings2", "tplDelete")}` }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(2));
    expect(lastRequest(client)).toMatchObject({ kind: "template-delete", name: "mine", input: "{}" });

    // example copy
    fireEvent.click(await screen.findByRole("button", { name: `sample ${t("settings2", "exampleCopy")}` }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(3));
    expect(lastRequest(client)).toMatchObject({ kind: "example-copy", name: "sample", input: "{}" });
  });

  it("ExpiredAskOffersExtendStopForever", async () => {
    const client = setup({
      status: "expired_ask",
      application: { id: "x", template: "mine", until: "2026-11-01T00:00:00+09:00", on_expire: "ask", reason: "" },
      templates: { mine: { name: "mine", description: "", tiers: {} } },
    });
    const extend = await screen.findByRole("button", { name: t("settings2", "answerExtend") });
    const stop = screen.getByRole("button", { name: t("settings2", "answerStop") });
    const forever = screen.getByRole("button", { name: t("settings2", "answerForever") });

    // extend cannot be sent without a new until
    expect(extend).toBeDisabled();
    expect(screen.getByText(t("settings2", "answerNeedUntil"))).toBeInTheDocument();
    fireEvent.click(extend);
    expect(client.saveSetting).not.toHaveBeenCalled();

    fireEvent.click(stop);
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    expect(lastRequest(client)).toMatchObject({ kind: "application-answer", name: "" });
    expect(JSON.parse(lastRequest(client).input)).toEqual({ choice: "stop" });

    fireEvent.click(await screen.findByRole("button", { name: t("settings2", "answerForever") }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(2));
    expect(JSON.parse(lastRequest(client).input)).toEqual({ choice: "forever" });
    expect(forever).toBeDefined();

    fireEvent.change(await screen.findByLabelText(t("settings2", "answerUntil")), {
      target: { value: "2026-11-20T10:00" },
    });
    const extendNow = await screen.findByRole("button", { name: t("settings2", "answerExtend") });
    await waitFor(() => expect(extendNow).toBeEnabled());
    fireEvent.click(extendNow);
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(3));
    const sent = JSON.parse(lastRequest(client).input) as { choice: string; until: string };
    expect(sent.choice).toBe("extend");
    expect(sent.until).toMatch(/^2026-11-20T10:00:00[+-]\d\d:\d\d$/);
  });

  it("RejectedShowsProblems", async () => {
    const client = setup(
      { templates: {} },
      result({ Outcome: "rejected", Problems: ["tiers.haiku: runner is unknown"] }),
    );
    fireEvent.click(await screen.findByRole("button", { name: t("settings2", "tplNew") }));
    const editor = await screen.findByRole("region", { name: t("settings2", "editorNewTitle") });
    fireEvent.change(within(editor).getByRole("textbox", { name: t("settings2", "fieldName") }), {
      target: { value: "bad" },
    });
    fireEvent.click(within(editor).getByRole("button", { name: t("settings", "save") }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    expect(await within(editor).findByText("tiers.haiku: runner is unknown")).toBeInTheDocument();
    expect(within(editor).getByText(t("settings", "problemsTitle"))).toBeInTheDocument();
    expect(within(editor).getByText(t("settings", "stateRejected"))).toBeInTheDocument();
  });
});
