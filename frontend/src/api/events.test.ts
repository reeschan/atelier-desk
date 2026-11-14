import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createWailsEvents, watchConnection } from "./events";
import type { DeskClient } from "./client";
import type { Connection } from "./types";
import { createFakeClient } from "../test/fakeClient";

function conn(version: string): Connection {
  return {
    state: "ready",
    version,
    python: "",
    pythonVersion: "",
    home: "",
    code: "",
    diagnostic: "",
    saving: false,
  } as unknown as Connection;
}

describe("WatchConnectionDeliversLatest", () => {
  it("hands the current connection() to cb when no event has come", async () => {
    const fake = createFakeClient({
      connection: vi.fn(() => Promise.resolve(conn("1.0"))),
    });
    const cb = vi.fn();
    watchConnection(fake as unknown as DeskClient, cb);
    await vi.waitFor(() => expect(cb).toHaveBeenCalledTimes(1));
    expect(cb).toHaveBeenLastCalledWith(conn("1.0"));
  });

  it("an event fired before connection() answers wins over the older answer", async () => {
    let handler: ((c: Connection) => void) | null = null;
    let resolveConn: (c: Connection) => void = () => {};
    const fake = createFakeClient({
      connection: vi.fn(
        () =>
          new Promise<Connection>((resolve) => {
            resolveConn = resolve;
          }),
      ),
    });
    fake.events.onConnectionChanged.mockImplementation((cb) => {
      handler = cb;
      return () => {};
    });
    const cb = vi.fn();
    watchConnection(fake as unknown as DeskClient, cb);

    handler!(conn("2.0"));
    resolveConn(conn("1.0"));
    await Promise.resolve();
    await Promise.resolve();

    expect(cb).toHaveBeenCalledTimes(1);
    expect(cb).toHaveBeenLastCalledWith(conn("2.0"));
  });

  it("delivers later events after the first answer, and the last one is the latest", async () => {
    let handler: ((c: Connection) => void) | null = null;
    const fake = createFakeClient({
      connection: vi.fn(() => Promise.resolve(conn("1.0"))),
    });
    fake.events.onConnectionChanged.mockImplementation((cb) => {
      handler = cb;
      return () => {};
    });
    const cb = vi.fn();
    watchConnection(fake as unknown as DeskClient, cb);
    await vi.waitFor(() => expect(cb).toHaveBeenCalledTimes(1));

    handler!(conn("3.0"));
    expect(cb).toHaveBeenCalledTimes(2);
    expect(cb).toHaveBeenLastCalledWith(conn("3.0"));
  });

  it("stops delivering after the returned unsubscribe is called", async () => {
    let handler: ((c: Connection) => void) | null = null;
    let resolveConn: (c: Connection) => void = () => {};
    const fake = createFakeClient({
      connection: vi.fn(
        () =>
          new Promise<Connection>((resolve) => {
            resolveConn = resolve;
          }),
      ),
    });
    fake.events.onConnectionChanged.mockImplementation((cb) => {
      handler = cb;
      return () => {};
    });
    const cb = vi.fn();
    const stop = watchConnection(fake as unknown as DeskClient, cb);
    stop();

    resolveConn(conn("1.0"));
    handler!(conn("2.0"));
    await Promise.resolve();
    await Promise.resolve();

    expect(cb).not.toHaveBeenCalled();
  });

  it("does not throw when connection() rejects", async () => {
    const fake = createFakeClient({
      connection: vi.fn(() => Promise.reject(new Error("boom"))),
    });
    const cb = vi.fn();
    expect(() => watchConnection(fake as unknown as DeskClient, cb)).not.toThrow();
    await Promise.resolve();
    await Promise.resolve();
    expect(cb).not.toHaveBeenCalled();
  });
});

type Listener = (data: unknown) => void;

describe("CreateWailsEventsUnsubscribe", () => {
  const listeners = new Map<string, Set<Listener>>();
  const w = window as unknown as { runtime?: unknown };
  let saved: unknown;

  function emit(name: string, data: unknown) {
    for (const l of [...(listeners.get(name) ?? [])]) l(data);
  }

  beforeEach(() => {
    listeners.clear();
    saved = w.runtime;
    const on = (name: string, cb: Listener): (() => void) => {
      let set = listeners.get(name);
      if (!set) {
        set = new Set();
        listeners.set(name, set);
      }
      set.add(cb);
      return () => {
        set.delete(cb);
      };
    };
    w.runtime = {
      EventsOn: on,
      EventsOnMultiple: (name: string, cb: Listener) => on(name, cb),
    };
  });

  afterEach(() => {
    w.runtime = saved;
  });

  it("delivers events to the callback while subscribed", () => {
    const events = createWailsEvents();
    const cb = vi.fn();
    events.onHomeChanged(cb);
    emit("home:changed", { paths: ["a"] });
    expect(cb).toHaveBeenCalledTimes(1);
    expect(cb).toHaveBeenCalledWith({ paths: ["a"] });
  });

  it("stops calling the callback after the unsubscribe function is called", () => {
    const events = createWailsEvents();
    const home = vi.fn();
    const job = vi.fn();
    const connection = vi.fn();
    const offHome = events.onHomeChanged(home);
    const offJob = events.onJobOutput(job);
    const offConn = events.onConnectionChanged(connection);

    offHome();
    offJob();
    offConn();

    emit("home:changed", { paths: ["a"] });
    emit("job:output", { id: "1", stream: "stdout", line: "x" });
    emit("connection:changed", conn("1.0"));

    expect(home).not.toHaveBeenCalled();
    expect(job).not.toHaveBeenCalled();
    expect(connection).not.toHaveBeenCalled();
  });

  it("stops the callback even when the runtime keeps delivering events", () => {
    // A runtime whose own unsubscribe does nothing: the callback must still stop.
    w.runtime = {
      EventsOn: (name: string, cb: Listener) => {
        listeners.set(name, new Set([cb]));
        return () => {};
      },
      EventsOnMultiple: (name: string, cb: Listener) => {
        listeners.set(name, new Set([cb]));
        return () => {};
      },
    };
    const events = createWailsEvents();
    const cb = vi.fn();
    const off = events.onHomeChanged(cb);
    emit("home:changed", { paths: [] });
    expect(cb).toHaveBeenCalledTimes(1);
    off();
    emit("home:changed", { paths: [] });
    expect(cb).toHaveBeenCalledTimes(1);
  });

  it("unsubscribing one subscription leaves another on the same event running", () => {
    const events = createWailsEvents();
    const a = vi.fn();
    const b = vi.fn();
    const offA = events.onHomeChanged(a);
    events.onHomeChanged(b);
    offA();
    emit("home:changed", { paths: [] });
    expect(a).not.toHaveBeenCalled();
    expect(b).toHaveBeenCalledTimes(1);
  });
});
