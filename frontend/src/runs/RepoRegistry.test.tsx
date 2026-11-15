import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { runs as T } from "../i18n/ja/runs";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import { RepoRegistry } from "./RepoRegistry";

describe("RepoRegistry", () => {
  it("AddAndRemoveRepo", async () => {
    const client = createFakeClient();
    client.repos.mockResolvedValue(["/work/a"]);
    client.addRepo.mockResolvedValue(["/work/a", "/work/b"]);
    client.removeRepo.mockResolvedValue(["/work/b"]);
    renderWithClient(<RepoRegistry />, client);

    expect(await screen.findByText("/work/a")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(T.repoPathLabel), { target: { value: "/work/b" } });
    fireEvent.click(screen.getByRole("button", { name: T.repoAdd }));
    expect(await screen.findByText("/work/b")).toBeInTheDocument();
    expect(client.addRepo).toHaveBeenCalledWith("/work/b");
    expect(screen.getByText("/work/a")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: T.repoRemoveLabel.replace("{dir}", "/work/a") }));
    await waitFor(() => expect(screen.queryByText("/work/a")).not.toBeInTheDocument());
    expect(client.removeRepo).toHaveBeenCalledWith("/work/a");
    expect(screen.getByText("/work/b")).toBeInTheDocument();
  });
});
