import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { DeskClient } from "../api/client";
import type { Connection, HomeView } from "../api/types";
import { settings } from "../i18n/ja/settings";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { SettingsScreen } from "./SettingsScreen";

type HomeCb = Parameters<DeskClient["events"]["onHomeChanged"]>[0];
type ConnCb = Parameters<DeskClient["events"]["onConnectionChanged"]>[0];

function runner() {
  return {
    enabled: true,
    path: "/bin/x",
    version: "1",
    checked_at: "",
    billing: "",
    selected: null,
    missing_selected: [],
    models: { fetched_at: "", method: "cli", list: [{ id: "m1", name: "" }] },
  };
}

function view(names: string[]): HomeView {
  const runners: Record<string, ReturnType<typeof runner>> = {};
  for (const n of names) runners[n] = runner();
  return {
    state: { home: "/home", runners, warnings: [] },
    base: { Files: {} },
  } as unknown as HomeView;
}

describe("SettingsScreen", () => {
  it("ReloadsOnHomeChangedWhileViewing", async () => {
    const client = createFakeClient();
    const cbs: HomeCb[] = [];
    client.events.onHomeChanged.mockImplementation((cb) => {
      cbs.push(cb);
      return () => {};
    });
    client.homeState.mockImplementation(() => Promise.resolve(view(["codex"])));
    renderWithClient(<SettingsScreen />, client);
    await screen.findByRole("article", { name: "codex" });
    expect(screen.queryByRole("article", { name: "gemini" })).not.toBeInTheDocument();

    client.homeState.mockImplementation(() => Promise.resolve(view(["codex", "gemini"])));
    act(() => {
      cbs.forEach((cb) => cb({ paths: ["agents.json"] }));
    });
    expect(await screen.findByRole("article", { name: "gemini" })).toBeInTheDocument();
  });

  it("SaveDisabledWhenNotReady", async () => {
    const client = createFakeClient();
    const connCbs: ConnCb[] = [];
    client.events.onConnectionChanged.mockImplementation((cb) => {
      connCbs.push(cb);
      return () => {};
    });
    client.homeState.mockImplementation(() => Promise.resolve(view(["codex"])));
    renderWithClient(<SettingsScreen />, client);
    await screen.findByRole("article", { name: "codex" });

    fireEvent.change(screen.getByLabelText(settings.billing), { target: { value: "subscription" } });
    expect(screen.getByRole("button", { name: settings.save })).toBeEnabled();

    act(() => {
      connCbs.forEach((cb) => cb({ state: "checking" } as unknown as Connection));
    });
    await waitFor(() => expect(screen.getByRole("button", { name: settings.save })).toBeDisabled());
  });
});
