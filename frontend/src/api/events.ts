import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { DeskClient } from "./client";
import type { Connection } from "./types";

export const EVENT_HOME_CHANGED = "home:changed";
export const EVENT_JOB_OUTPUT = "job:output";
export const EVENT_CONNECTION_CHANGED = "connection:changed";

export interface DeskEvents {
  onHomeChanged(cb: (e: { paths: string[] }) => void): () => void;
  onJobOutput(cb: (e: { id: string; stream: string; line: string }) => void): () => void;
  onConnectionChanged(cb: (c: Connection) => void): () => void;
}

/** Subscribes to a Wails event. The returned function stops the callback. */
function subscribe<T>(name: string, cb: (data: T) => void): () => void {
  let live = true;
  const off = EventsOn(name, (data: T) => {
    if (live) cb(data);
  });
  return () => {
    live = false;
    off();
  };
}

export function createWailsEvents(): DeskEvents {
  return {
    onHomeChanged: (cb) => subscribe(EVENT_HOME_CHANGED, cb),
    onJobOutput: (cb) => subscribe(EVENT_JOB_OUTPUT, cb),
    onConnectionChanged: (cb) => subscribe(EVENT_CONNECTION_CHANGED, cb),
  };
}

/**
 * Subscribes to connection:changed first, then reads connection() once and
 * hands it to cb, so an event fired before the subscription is not missed.
 * An event that arrives while connection() is pending is newer than its
 * answer, so the answer is dropped then. Returns the unsubscribe function.
 */
export function watchConnection(
  client: DeskClient,
  cb: (c: Connection) => void,
): () => void {
  let live = true;
  let sawEvent = false;
  const off = client.events.onConnectionChanged((c) => {
    if (!live) return;
    sawEvent = true;
    cb(c);
  });
  client
    .connection()
    .then((c) => {
      if (live && !sawEvent) cb(c);
    })
    .catch(() => {
      // The connection state itself arrives as connection:changed.
    });
  return () => {
    live = false;
    off();
  };
}
