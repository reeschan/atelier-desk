import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { CasesView } from "../api/types";
import { runs as T } from "../i18n/ja/runs";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { CaseTable } from "./CaseTable";

describe("CaseTable", () => {
  it("CauseCheckFiltersCases", async () => {
    const client = createFakeClient();
    client.cases.mockResolvedValue({
      cases: [
        { Repo: "/r/a", WorkKey: "w", PBI: "PBI-001", Causes: ["spec"], Summary: "summary-spec", Archived: false },
        { Repo: "/r/a", WorkKey: "w", PBI: "PBI-002", Causes: ["env"], Summary: "summary-env", Archived: false },
        { Repo: "/r/a", WorkKey: "w", PBI: "PBI-003", Causes: ["spec", "flaky"], Summary: "summary-both", Archived: false },
      ],
      warnings: [],
    } as unknown as CasesView);
    renderWithClient(<CaseTable />, client);

    expect(await screen.findByText("summary-spec")).toBeInTheDocument();
    expect(screen.getByText("summary-env")).toBeInTheDocument();
    expect(screen.getByText("summary-both")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: T.causes.spec }));
    expect(screen.getByText("summary-spec")).toBeInTheDocument();
    expect(screen.getByText("summary-both")).toBeInTheDocument();
    expect(screen.queryByText("summary-env")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: T.causes.spec }));
    expect(screen.getByText("summary-env")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: T.causes.env }));
    expect(screen.getByText("summary-env")).toBeInTheDocument();
    expect(screen.queryByText("summary-spec")).not.toBeInTheDocument();
    expect(screen.queryByText("summary-both")).not.toBeInTheDocument();
  });
});
