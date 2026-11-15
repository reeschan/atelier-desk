import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { HomeState, HomeView, SaveResult } from "../api/types";
import { settings } from "../i18n/ja/settings";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { AgentsTab } from "./AgentsTab";

const state = {
  home: "/home",
  runners: {
    codex: {
      enabled: true,
      path: "/bin/codex",
      version: "1.0",
      checked_at: "2026-01-01",
      billing: "",
      selected: null,
      missing_selected: [],
      models: {
        fetched_at: "2026-01-01",
        method: "cli",
        list: [{ id: "gpt-x", name: "", efforts: ["low"] }],
      },
    },
  },
  warnings: [],
} as unknown as HomeState;

const base = { Files: { "agents.json": "v1" } };

describe("AgentsTab", () => {
  it("RunnerDisableAndManualModelKinds", async () => {
    const client = createFakeClient();
    client.homeState.mockImplementation(() =>
      Promise.resolve({ state, base } as unknown as HomeView),
    );
    client.saveSetting.mockImplementation(() =>
      Promise.resolve({ Outcome: "saved", Base: base } as unknown as SaveResult),
    );
    renderWithClient(<AgentsTab state={state} base={base as never} ready />, client);

    // disable the runner and save: kind runner, name <runner>
    const enabled = await screen.findByRole("checkbox", { name: `codex ${settings.runnerEnabled}` });
    fireEvent.click(enabled);
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(1));
    const first = client.saveSetting.mock.calls[0][0];
    expect(first.kind).toBe("runner");
    expect(first.name).toBe("codex");
    expect(JSON.parse(first.input)).toMatchObject({ enabled: false });
    await screen.findByText(settings.saved);

    // add a model by hand: kind model, name <runner>
    fireEvent.change(screen.getByLabelText(settings.addModelId), { target: { value: "my-model" } });
    fireEvent.change(screen.getByLabelText(settings.addModelEfforts), {
      target: { value: "low, high" },
    });
    fireEvent.click(screen.getByRole("button", { name: settings.addModelButton }));
    await waitFor(() => expect(client.saveSetting).toHaveBeenCalledTimes(2));
    const second = client.saveSetting.mock.calls[1][0];
    expect(second.kind).toBe("model");
    expect(second.name).toBe("codex");
    expect(JSON.parse(second.input)).toEqual({ id: "my-model", efforts: ["low", "high"] });
  });
});
