import { render, screen } from "@testing-library/react";
import { vi } from "vitest";
import { describe, expect, it } from "vitest";
import type { SaveResult } from "../api/types";
import { settings } from "../i18n/ja/settings";
import { SaveBar } from "./SaveBar";
import type { SaveBarProps } from "./SaveBar";

function props(result: SaveResult): SaveBarProps["save"] {
  return {
    state: "viewing",
    problems: [],
    changed: false,
    result,
    error: "",
    save: vi.fn(() => Promise.resolve(null)),
    overwrite: vi.fn(() => Promise.resolve(null)),
    reload: vi.fn(() => Promise.resolve()),
    discard: vi.fn(),
  };
}

describe("SaveBar", () => {
  it("PartialFailureShowsBackupAndRecovery", () => {
    const result = {
      Outcome: "partial_failure",
      Failed: "routing.json",
      BackupDir: "/home/.atelier/backup/20260101",
      RecoveryFile: "/home/.atelier/recovery-20260101.json",
      Problems: [],
    } as unknown as SaveResult;
    render(<SaveBar save={props(result)} ready />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(settings.partialTitle);
    expect(alert).toHaveTextContent(settings.backupDir);
    expect(alert).toHaveTextContent("/home/.atelier/backup/20260101");
    expect(alert).toHaveTextContent(settings.recoveryFile);
    expect(alert).toHaveTextContent("/home/.atelier/recovery-20260101.json");
    expect(alert).toHaveTextContent("routing.json");
    expect(screen.queryByText(settings.saved)).not.toBeInTheDocument();
  });
});
