import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { HomeState, HomeView, SaveResult, WorkflowView } from "../api/types";
import { t } from "../i18n";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { WorkflowsTab } from "./WorkflowsTab";

const BASE = { Taken: null, Files: { "workflows/mine.json": "aaa" } };

describe("WorkflowsTab", () => {
  it("DeleteOwnWorkflowShowsBundledReturns", async () => {
    const own = {
      workflow: {
        name: "mine",
        source: "home",
        bundled: true,
        data: { name: "mine", description: { ja: "x", en: "x" }, stages: {} },
      },
    } as unknown as WorkflowView;
    const client = createFakeClient();
    client.workflows.mockResolvedValue([own]);
    client.components.mockResolvedValue([]);
    client.homeState.mockResolvedValue({ state: {}, base: BASE } as unknown as HomeView);
    client.saveSetting.mockResolvedValue({
      Outcome: "saved",
      Base: BASE,
      Current: BASE,
      Problems: [],
    } as unknown as SaveResult);
    renderWithClient(<WorkflowsTab state={{} as HomeState} base={BASE} ready={true} />, client);

    const editor = await screen.findByRole("region", { name: "mine" });
    await waitFor(() =>
      expect(within(editor).getByText(t("settings2", "wfDeleteBack"))).toBeInTheDocument(),
    );
    fireEvent.click(within(editor).getByRole("button", { name: t("settings2", "wfDelete") }));

    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    const req = client.saveSetting.mock.calls[0][0];
    expect(req).toMatchObject({ kind: "workflow-delete", name: "mine", input: "{}" });
    expect(await screen.findByText(t("settings2", "wfDeleted"))).toBeInTheDocument();
  });
});
