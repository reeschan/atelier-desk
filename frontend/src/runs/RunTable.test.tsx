import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Run, RunsView } from "../api/types";
import { runs as T } from "../i18n/ja/runs";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { RunTable } from "./RunTable";

function run(p: Partial<Record<string, unknown>>): Run {
  return { Repo: "", WorkKey: "", FirstAt: "", LastAt: "", Archived: false, Source: "", PBIs: [], ...p } as unknown as Run;
}

function view(list: Run[]): RunsView {
  return { runs: list, warnings: [] } as unknown as RunsView;
}

describe("RunTable", () => {
  it("RunsOrderedWithArchivedMark", async () => {
    const client = createFakeClient();
    client.runs.mockResolvedValue(
      view([
        run({ Repo: "/r/b", WorkKey: "b-old", LastAt: "2026-11-01T10:00:00+09:00" }),
        run({ Repo: "/r/a", WorkKey: "a-old", LastAt: "2026-11-02T10:00:00+09:00" }),
        run({ Repo: "/r/a", WorkKey: "a-new", LastAt: "2026-11-10T10:00:00+09:00", Archived: true }),
        run({ Repo: "/r/b", WorkKey: "b-new", LastAt: "2026-11-05T10:00:00+09:00" }),
      ]),
    );
    const { container } = renderWithClient(<RunTable />, client);
    await screen.findByText("a-new");

    const items = Array.from(container.querySelectorAll("li[data-repo]"));
    expect(items.map((li) => li.getAttribute("data-repo"))).toEqual(["/r/a", "/r/a", "/r/b", "/r/b"]);
    expect(items.map((li) => within(li as HTMLElement).getAllByText(/-(old|new)$/)[0].textContent)).toEqual([
      "a-new",
      "a-old",
      "b-new",
      "b-old",
    ]);
    expect(items.map((li) => li.getAttribute("data-archived"))).toEqual(["true", "false", "false", "false"]);
    expect(within(items[0] as HTMLElement).getByText(T.archived)).toBeInTheDocument();
    expect(within(items[1] as HTMLElement).queryByText(T.archived)).not.toBeInTheDocument();
  });

  it("OpenRowShowsPbiResultsAndCauses", async () => {
    const client = createFakeClient();
    client.runs.mockResolvedValue(
      view([
        run({
          Repo: "/r/a",
          WorkKey: "w-1",
          LastAt: "2026-11-10T10:00:00+09:00",
          PBIs: [
            { ID: "PBI-001", Passed: true, Reworks: 0, Escalations: 0, Held: false, HoldReason: "", Model: "m-pass", Attempts: [] },
            {
              ID: "PBI-002",
              Passed: false,
              Reworks: 2,
              Escalations: 1,
              Held: false,
              HoldReason: "",
              Model: "m-fail",
              Attempts: [{ Outcome: "FAIL", Causes: ["spec", "env"] }],
            },
            { ID: "PBI-003", Passed: false, Reworks: 0, Escalations: 0, Held: true, HoldReason: "scope", Model: "m-held", Attempts: [] },
          ],
        }),
      ]),
    );
    renderWithClient(<RunTable />, client);

    expect(await screen.findByText(T.resultsHint)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /w-1/ }));

    const results = await screen.findByRole("region", { name: T.resultsTitle });
    const rows = within(results).getAllByRole("row");
    const row = (id: string) => rows.find((r) => within(r).queryByText(id) !== null) as HTMLElement;

    expect(within(row("PBI-001")).getByText(T.resultPassed)).toBeInTheDocument();
    expect(within(row("PBI-002")).getByText("FAIL")).toBeInTheDocument();
    expect(within(row("PBI-002")).getByText(`${T.causes.spec} · ${T.causes.env}`)).toBeInTheDocument();
    expect(within(row("PBI-002")).getByText(T.reworkCount.replace("{n}", "2"))).toBeInTheDocument();
    expect(within(row("PBI-002")).getByText(T.escalationCount.replace("{n}", "1"))).toBeInTheDocument();
    expect(within(row("PBI-003")).getByText(T.resultHeld)).toBeInTheDocument();
    expect(within(row("PBI-003")).getByText(T.holdReason.replace("{reason}", T.causes.scope))).toBeInTheDocument();
  });
});
