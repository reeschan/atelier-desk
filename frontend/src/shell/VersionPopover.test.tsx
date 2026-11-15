import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeskError } from "../api/client";
import type { Connection } from "../api/types";
import { errorText, t } from "../i18n";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { Shell } from "./Shell";
import { VersionPopover } from "./VersionPopover";

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

describe("VersionPopover", () => {
  it("SwitchBusySavingKeepsVersion: the busy_saving text shows and the version stays", async () => {
    const client = createFakeClient();
    client.versions.mockResolvedValue(["1.0", "2.0"]);
    client.switchVersion.mockRejectedValue(new DeskError("busy_saving", ""));
    const onSwitched = vi.fn();
    renderWithClient(<VersionPopover version="1.0" disabled={false} onSwitched={onSwitched} />, client);

    fireEvent.click(screen.getByRole("button", { name: t("common", "versionButtonLabel") }));
    fireEvent.click(await screen.findByRole("button", { name: /2\.0/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent(errorText("busy_saving"));
    expect(client.switchVersion).toHaveBeenCalledWith("2.0");
    expect(onSwitched).not.toHaveBeenCalled();
    expect(screen.getByTestId("current-version")).toHaveTextContent("1.0");
  });

  it("SwitchBusySavingKeepsVersion: in the shell the version display and badge are unchanged", async () => {
    const client = createFakeClient();
    client.connection.mockResolvedValue(conn({ version: "1.0" }));
    client.versions.mockResolvedValue(["1.0", "2.0"]);
    client.switchVersion.mockRejectedValue(new DeskError("busy_saving", ""));
    renderWithClient(<Shell />, client);

    await waitFor(() => expect(screen.getByTestId("current-version")).toHaveTextContent("1.0"));
    fireEvent.click(screen.getByRole("button", { name: t("common", "versionButtonLabel") }));
    fireEvent.click(await screen.findByRole("button", { name: /2\.0/ }));

    expect(await screen.findByText(errorText("busy_saving"))).toBeInTheDocument();
    expect(screen.getByTestId("current-version")).toHaveTextContent("1.0");
    expect(screen.getByRole("status", { name: t("common", "connLabel") })).toHaveTextContent(
      t("common", "connReady"),
    );
  });
});
