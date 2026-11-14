import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { DeskClient } from "../api/client";
import { useClient } from "../api/client";
import type { Baseline, HomeView, SaveResult } from "../api/types";
import { settings } from "../i18n/ja/settings";
import { createFakeClient, renderWithClient } from "../test/fakeClient";
import type { FakeClient } from "../test/fakeClient";
import { SaveBar } from "./SaveBar";
import { useHomeSave } from "./useHomeSave";

type HomeChangedCb = Parameters<DeskClient["events"]["onHomeChanged"]>[0];

const FILE = "agents.json";

function baseOf(ver: number): Baseline {
  return { Files: { [FILE]: `v${ver}` } } as unknown as Baseline;
}

/** A tiny home: one file whose content is shown as state.home. The fake client saves only when the base matches. */
function makeHome(opts: { rejectWith?: string[] } = {}) {
  const home = { content: "A", ver: 1, cbs: [] as HomeChangedCb[] };
  const client: FakeClient = createFakeClient();
  client.homeState.mockImplementation(() =>
    Promise.resolve({ state: { home: home.content }, base: baseOf(home.ver) } as unknown as HomeView),
  );
  client.saveSetting.mockImplementation((req) => {
    if (opts.rejectWith !== undefined) {
      return Promise.resolve({ Outcome: "rejected", Problems: opts.rejectWith } as unknown as SaveResult);
    }
    if ((req.base as Baseline).Files?.[FILE] !== `v${home.ver}`) {
      return Promise.resolve({ Outcome: "external_change", Current: baseOf(home.ver) } as unknown as SaveResult);
    }
    home.content = req.input;
    home.ver++;
    return Promise.resolve({ Outcome: "saved", Base: baseOf(home.ver) } as unknown as SaveResult);
  });
  client.events.onHomeChanged.mockImplementation((cb) => {
    home.cbs.push(cb);
    return () => {};
  });
  return { home, client };
}

function Harness() {
  const client = useClient();
  const h = useHomeSave<string>({
    kind: "runner",
    name: "x",
    load: async () => {
      const v = await client.homeState();
      return { value: v.state.home, base: v.base };
    },
    serialize: (v) => v,
    watches: (paths) => paths.some((p) => p === FILE),
  });
  return (
    <div>
      <span aria-label="draft">{h.draft ?? ""}</span>
      <button type="button" onClick={() => h.setDraft("edited")}>
        edit
      </button>
      <button type="button" onClick={() => h.setDraft("edited2")}>
        edit2
      </button>
      <SaveBar save={h} ready />
    </div>
  );
}

function fire(home: { cbs: HomeChangedCb[] }, paths: string[]) {
  act(() => {
    home.cbs.forEach((cb) => cb({ paths }));
  });
}

async function openAndEdit(env: ReturnType<typeof makeHome>) {
  renderWithClient(<Harness />, env.client);
  await waitFor(() => expect(screen.getByLabelText("draft")).toHaveTextContent("A"));
  fireEvent.click(screen.getByText("edit"));
}

describe("useHomeSave", () => {
  it("RejectedShowsProblemsNotSaved", async () => {
    const env = makeHome({ rejectWith: ["problem one", "problem two"] });
    await openAndEdit(env);
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    expect(await screen.findByText("problem one")).toBeInTheDocument();
    expect(screen.getByText("problem two")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(settings.stateRejected);
    expect(screen.queryByText(settings.saved)).not.toBeInTheDocument();
  });

  it("ExternalChangeAndOverwrite", async () => {
    // reload throws the edit away
    const env = makeHome();
    await openAndEdit(env);
    env.home.content = "B";
    env.home.ver = 2;
    fire(env.home, [FILE]);
    expect(await screen.findByText(settings.stateExternal)).toBeInTheDocument();
    expect(env.client.saveSetting).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: settings.reload }));
    await waitFor(() => expect(screen.getByLabelText("draft")).toHaveTextContent("B"));
    expect(screen.getByRole("status")).toHaveTextContent(settings.stateViewing);
    expect(env.client.saveSetting).not.toHaveBeenCalled();
  });

  it("ExternalChangeAndOverwrite overwrites with the home's present base", async () => {
    const env = makeHome();
    await openAndEdit(env);
    env.home.content = "B";
    env.home.ver = 2;
    fire(env.home, [FILE]);
    await screen.findByText(settings.stateExternal);
    fireEvent.click(screen.getByRole("button", { name: settings.overwrite }));
    await waitFor(() => expect(env.client.saveSetting).toHaveBeenCalledTimes(1));
    const req = env.client.saveSetting.mock.calls[0][0];
    expect((req.base as Baseline).Files[FILE]).toBe("v2");
    expect(req.input).toBe("edited");
    expect(req.kind).toBe("runner");
    expect(req.name).toBe("x");
    expect(await screen.findByText(settings.saved)).toBeInTheDocument();
    expect(env.home.content).toBe("edited");
  });

  it("ExternalChangeFromSaveResult", async () => {
    const env = makeHome();
    await openAndEdit(env);
    // the home changed outside and no home:changed arrived
    env.home.content = "B";
    env.home.ver = 2;
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    expect(await screen.findByText(settings.stateExternal)).toBeInTheDocument();
    expect(env.client.saveSetting).toHaveBeenCalledTimes(1);
    expect((env.client.saveSetting.mock.calls[0][0].base as Baseline).Files[FILE]).toBe("v1");
    expect(env.home.content).toBe("B");
    // only the overwrite button sends the answer's current as the base
    fireEvent.click(screen.getByRole("button", { name: settings.overwrite }));
    await waitFor(() => expect(env.client.saveSetting).toHaveBeenCalledTimes(2));
    expect((env.client.saveSetting.mock.calls[1][0].base as Baseline).Files[FILE]).toBe("v2");
    expect(await screen.findByText(settings.saved)).toBeInTheDocument();
    expect(env.home.content).toBe("edited");
  });

  it("OwnReplacementEventIsNotExternal", async () => {
    const env = makeHome();
    await openAndEdit(env);
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    expect(await screen.findByText(settings.saved)).toBeInTheDocument();
    // our own replacement of the file comes back as home:changed
    fire(env.home, [FILE]);
    await waitFor(() => expect(screen.getByLabelText("draft")).toHaveTextContent("edited"));
    expect(screen.queryByText(settings.stateExternal)).not.toBeInTheDocument();
    // a second save from the same screen goes through too
    fireEvent.click(screen.getByText("edit2"));
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    await waitFor(() => expect(env.client.saveSetting).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(env.home.content).toBe("edited2"));
    expect(screen.queryByText(settings.stateExternal)).not.toBeInTheDocument();
    expect(await screen.findByText(settings.saved)).toBeInTheDocument();
  });

  it("OwnReplacementEventIsNotExternal ignores a path the tab does not read", async () => {
    const env = makeHome();
    await openAndEdit(env);
    fire(env.home, ["routing.json"]);
    await waitFor(() => expect(env.client.homeState.mock.calls.length).toBeGreaterThan(1));
    expect(screen.queryByText(settings.stateExternal)).not.toBeInTheDocument();
    expect(screen.getByLabelText("draft")).toHaveTextContent("edited");
    fireEvent.click(screen.getByRole("button", { name: settings.save }));
    expect(await screen.findByText(settings.saved)).toBeInTheDocument();
    expect(env.home.content).toBe("edited");
  });
});
