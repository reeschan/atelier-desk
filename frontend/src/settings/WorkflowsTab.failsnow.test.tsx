import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { HomeState, HomeView, SaveResult, WorkflowView } from "../api/types";
import { t } from "../i18n";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { WorkflowsTab } from "./WorkflowsTab";

// fails now: WorkflowsTab.tsx:193 reads workflows() and homeState() side by side, so an
// outside change made between the two reads is inside the base but not in the form
// (repro: docs/pbi/tools/pbi016-workflow-base-race.mjs, user story 27).
const PATH = "workflows/jev-routed.json";

describe("WorkflowsTab", () => {
  it("OutsideEditBetweenReadsNotOverwritten", async () => {
    let file = JSON.stringify({ name: "jev-routed", description: { ja: "old", en: "" }, stages: {} });
    let release: () => void = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    const base = (f: string) => ({ Taken: null, Files: { [PATH]: f } });

    const client = createFakeClient();
    // reads the file as it is now
    client.workflows.mockImplementation(() =>
      Promise.resolve([
        {
          workflow: { name: "jev-routed", source: "home", bundled: false, data: JSON.parse(file) as unknown },
        } as unknown as WorkflowView,
      ]),
    );
    client.components.mockResolvedValue([]);
    // resolves later: the baseline is taken after the outside edit
    client.homeState.mockImplementation(async () => {
      await gate;
      return { state: {}, base: base(file) } as unknown as HomeView;
    });
    // the Go side: a save passes only when the base still matches the file
    client.saveSetting.mockImplementation((req) => {
      const same = req.base.Files[PATH] === file;
      if (!same) {
        return Promise.resolve({
          Outcome: "external_change",
          Current: base(file),
          Base: base(file),
          Problems: [],
        } as unknown as SaveResult);
      }
      file = req.input;
      return Promise.resolve({
        Outcome: "saved",
        Base: base(file),
        Current: base(file),
        Problems: [],
      } as unknown as SaveResult);
    });

    renderWithClient(<WorkflowsTab state={{} as HomeState} base={base(file)} ready={true} />, client);
    await waitFor(() => expect(client.homeState).toHaveBeenCalled());
    await waitFor(() => expect(client.workflows.mock.calls.length).toBeGreaterThanOrEqual(2));

    // the outside edit lands between the two reads
    const outside = JSON.stringify({ name: "jev-routed", description: { ja: "edited outside", en: "" }, stages: {} });
    file = outside;
    release();

    const editor = await screen.findByRole("region", { name: "jev-routed" });
    const desc = await within(editor).findByRole("textbox", { name: t("settings2", "wfDescJa") });
    fireEvent.change(desc, { target: { value: "mine" } });
    fireEvent.click(within(editor).getByRole("button", { name: t("settings", "save") }));

    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    // the save must be refused as external and the outside edit must survive
    await waitFor(() =>
      expect(within(editor).getByText(t("settings", "stateExternal"))).toBeInTheDocument(),
    );
    expect(file).toBe(outside);
  });
});
