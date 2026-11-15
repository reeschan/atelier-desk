import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import type { Connection } from "../api/types";
import { t } from "../i18n";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { Shell } from "./Shell";

function conn(over: Record<string, unknown>): Connection {
  return {
    state: "ready",
    version: "",
    python: "",
    pythonVersion: "",
    home: "",
    code: "",
    diagnostic: "",
    saving: false,
    ...over,
  } as unknown as Connection;
}

beforeEach(() => {
  window.location.hash = "";
});

// VersionPopover.tsx:76 calls onSwitched(answer) with whatever switchVersion answered,
// even when connection:changed (the finished check) already arrived and is newer.
describe("VersionPopover (fails now)", () => {
  it("SwitchedEventBeforeAnswerDoesNotLeaveChecking", async () => {
    const client = createFakeClient();
    let emit: (c: Connection) => void = () => {};
    client.events.onConnectionChanged.mockImplementation((cb) => {
      emit = cb;
      return () => {};
    });
    client.connection.mockResolvedValue(conn({ version: "1.0" }));
    client.versions.mockResolvedValue(["1.0", "2.0"]);
    let answer: (c: Connection) => void = () => {};
    client.switchVersion.mockImplementation(
      () =>
        new Promise<Connection>((resolve) => {
          answer = resolve;
        }),
    );
    renderWithClient(<Shell />, client);

    await waitFor(() => expect(screen.getByTestId("current-version")).toHaveTextContent("1.0"));
    fireEvent.click(screen.getByRole("button", { name: t("common", "versionButtonLabel") }));
    fireEvent.click(await screen.findByRole("button", { name: /2\.0/ }));
    await waitFor(() => expect(client.switchVersion).toHaveBeenCalledWith("2.0"));

    // The finished check (ready, 2.0) arrives before switchVersion's answer, which
    // still carries the checking state.
    act(() => emit(conn({ version: "2.0", state: "ready" })));
    await act(async () => {
      answer(conn({ version: "2.0", state: "checking" }));
    });

    await waitFor(() => expect(screen.getByTestId("current-version")).toHaveTextContent("2.0"));
    expect(screen.getByRole("status", { name: t("common", "connLabel") })).toHaveTextContent(
      t("common", "connReady"),
    );
  });
});
