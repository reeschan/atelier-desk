import { useEffect, useState } from "react";
import { screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useClient } from "../api/client";
import { createFakeClient, renderWithClient } from "./fakeClient";

function RepoList() {
  const client = useClient();
  const [repos, setRepos] = useState<string[] | null>(null);
  useEffect(() => {
    void client.repos().then(setRepos);
  }, [client]);
  if (repos === null) return <p>loading</p>;
  return (
    <ul>
      {repos.map((r) => (
        <li key={r}>{r}</li>
      ))}
    </ul>
  );
}

describe("test foundation", () => {
  it("renders with the fake client and calls its method", async () => {
    const client = createFakeClient({ repos: createFakeClient().repos.mockResolvedValue(["/a/b"]) });
    const r = renderWithClient(<RepoList />, client);
    expect(r.client).toBe(client);
    expect(await screen.findByText("/a/b")).toBeInTheDocument();
    expect(client.repos).toHaveBeenCalledTimes(1);
  });

  it("resolves an empty value by default", async () => {
    const { client } = renderWithClient(<RepoList />);
    await waitFor(() => expect(screen.queryByText("loading")).not.toBeInTheDocument());
    expect(client.repos).toHaveBeenCalled();
  });
});
