import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { Connection } from "../api/types";
import { errorText, t } from "../i18n";
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
afterEach(() => {
  window.location.hash = "";
});

describe("Shell", () => {
  it("PythonUnavailableShowsBadgeBannerAndInput", async () => {
    const client = createFakeClient();
    client.connection.mockResolvedValue(
      conn({ state: "python_unavailable", code: "python_not_found", diagnostic: "python: command not found" }),
    );
    client.setPythonPath.mockResolvedValue(conn({ state: "ready", version: "1.0", pythonVersion: "3.12" }));
    renderWithClient(<Shell />, client);

    await waitFor(() =>
      expect(screen.getByRole("status", { name: t("common", "connLabel") })).toHaveTextContent(
        t("common", "connPythonUnavailable"),
      ),
    );
    const banner = await screen.findByRole("region", { name: t("common", "bannerReadyLabel") });
    expect(banner).toHaveTextContent(t("common", "bannerPythonLead"));
    expect(banner).toHaveTextContent(errorText("python_not_found"));
    expect(banner).toHaveTextContent("python: command not found");
    expect(banner).toHaveTextContent(t("common", "fixInstallPython"));

    const input = screen.getByRole("textbox", { name: t("common", "pythonPathLabel") });
    fireEvent.change(input, { target: { value: "  C:\\Python312\\python.exe  " } });
    fireEvent.click(screen.getByRole("button", { name: t("common", "pythonPathApply") }));
    await waitFor(() => expect(client.setPythonPath).toHaveBeenCalledWith("C:\\Python312\\python.exe"));

    // The answer is the newest state: the banner goes and the badge reads ready.
    await waitFor(() =>
      expect(screen.getByRole("status", { name: t("common", "connLabel") })).toHaveTextContent(
        t("common", "connReady"),
      ),
    );
    expect(screen.queryByRole("region", { name: t("common", "bannerReadyLabel") })).toBeNull();
  });

  it("UnsupportedShowsLabel", async () => {
    const client = createFakeClient();
    client.connection.mockResolvedValue(conn({ state: "unsupported", version: "9.9", code: "" }));
    renderWithClient(<Shell />, client);

    await waitFor(() =>
      expect(screen.getByRole("status", { name: t("common", "connLabel") })).toHaveTextContent(
        t("common", "connUnsupported"),
      ),
    );
    const banner = await screen.findByRole("region", { name: t("common", "bannerReadyLabel") });
    expect(banner).toHaveTextContent(t("common", "bannerUnsupportedLead"));
    expect(banner).toHaveTextContent(errorText("unsupported"));
    // No Python path form for an unsupported version.
    expect(screen.queryByRole("textbox", { name: t("common", "pythonPathLabel") })).toBeNull();
  });

  it("VersionChangeEventUpdatesDisplay", async () => {
    const client = createFakeClient();
    let emit: (c: Connection) => void = () => {};
    client.events.onConnectionChanged.mockImplementation((cb) => {
      emit = cb;
      return () => {};
    });
    client.connection.mockResolvedValue(conn({ version: "1.0" }));
    renderWithClient(<Shell />, client);

    await waitFor(() => expect(screen.getByTestId("current-version")).toHaveTextContent("1.0"));
    act(() => emit(conn({ version: "2.0" })));
    expect(screen.getByTestId("current-version")).toHaveTextContent("2.0");
    expect(screen.getByTestId("current-version")).not.toHaveTextContent("1.0");
  });
});
