import { useEffect, useRef, useState } from "react";
import { toDeskError, useClient } from "../api/client";
import type { Baseline, HomeState, SaveResult } from "../api/types";

/** The state diagram of "editing and saving one personal setting" (spec). */
export type SaveState =
  | "viewing"
  | "editing"
  | "checking"
  | "writing"
  | "rejected"
  | "external";

/** What SettingsScreen hands to every tab. */
export type TabProps = { state: HomeState; base: Baseline; ready: boolean };

/** Another kind / name / input than the hook's own, for one save call. */
export type SaveOverride = { kind?: string; name?: string; input?: string };

export type HomeSaveOptions<T> = {
  kind: string;
  name?: string;
  load: () => Promise<{ value: T; base: Baseline }>;
  serialize: (v: T) => string;
  /** True when a changed home path (home-relative, "/"-separated) is one this tab reads. Omitted: every path. */
  watches?: (paths: string[]) => boolean;
  /** False while Atelier cannot be asked: save and overwrite do nothing. Omitted: true. */
  ready?: boolean;
};

export type HomeSave<T> = {
  state: SaveState;
  /** What the home holds (null until the first load). */
  value: T | null;
  /** What the form shows and edits. */
  draft: T | null;
  setDraft: (d: T | ((prev: T) => T)) => void;
  save: (override?: SaveOverride) => Promise<SaveResult | null>;
  overwrite: () => Promise<SaveResult | null>;
  reload: () => Promise<void>;
  discard: () => void;
  /** Atelier's problems of the last rejected save, as it wrote them. */
  problems: string[];
  /** True when the draft differs from the value. */
  changed: boolean;
  /** The last save's answer (saved, external_change, partial_failure, rejected). */
  result: SaveResult | null;
  /** DeskError code of the last failed call (a rejected saveSetting or a failed load); "" if none. */
  error: string;
};

type Request = { kind: string; name: string; input: string };

/**
 * One setting's edit-and-save state machine on the screen side. A save sends
 * the base the form was opened on; only overwrite() sends the `current` that an
 * external_change answer carried, and only from the external state.
 */
export function useHomeSave<T>(opts: HomeSaveOptions<T>): HomeSave<T> {
  const client = useClient();
  const optsRef = useRef(opts);
  optsRef.current = opts;

  const [state, setStateView] = useState<SaveState>("viewing");
  const [value, setValueView] = useState<T | null>(null);
  const [draft, setDraftView] = useState<T | null>(null);
  const [problems, setProblems] = useState<string[]>([]);
  const [result, setResult] = useState<SaveResult | null>(null);
  const [error, setError] = useState("");

  const stateRef = useRef<SaveState>("viewing");
  const valueRef = useRef<T | null>(null);
  const draftRef = useRef<T | null>(null);
  const baseRef = useRef<Baseline | null>(null);
  const currentRef = useRef<Baseline | null>(null);
  const lastRef = useRef<(Request & { standalone: boolean }) | null>(null);
  const seqRef = useRef(0);
  const aliveRef = useRef(true);

  function go(s: SaveState): void {
    stateRef.current = s;
    setStateView(s);
  }

  function apply(r: { value: T; base: Baseline }): void {
    valueRef.current = r.value;
    draftRef.current = r.value;
    baseRef.current = r.base;
    setValueView(r.value);
    setDraftView(r.value);
    setError("");
  }

  /** load(), dropped when a newer load or a save started meanwhile. */
  async function loadFresh(): Promise<{ value: T; base: Baseline } | null> {
    const my = ++seqRef.current;
    try {
      const r = await optsRef.current.load();
      return aliveRef.current && my === seqRef.current ? r : null;
    } catch (e) {
      if (aliveRef.current && my === seqRef.current) setError(toDeskError(e).code);
      return null;
    }
  }

  /**
   * Reads again. An edit that began while the read was pending is never
   * overwritten: the draft stays and the state becomes external (or stays
   * external). The read applies only while the state is viewing. `force` only
   * means the caller already moved to viewing; it never widens that condition.
   */
  async function refresh(_force: boolean): Promise<void> {
    const r = await loadFresh();
    if (r === null) return;
    const s = stateRef.current;
    if (s === "editing" || s === "rejected") {
      currentRef.current = null;
      go("external");
    } else if (s === "viewing") apply(r);
  }

  /**
   * A path this tab does not read changed: renew only the fingerprints of the
   * paths it does not read. The paths it reads keep the old fingerprint, so an
   * outside change to them (even one whose home:changed was dropped while
   * checking) still comes back from the next save as external_change.
   */
  async function refreshBase(): Promise<void> {
    const r = await loadFresh();
    const s = stateRef.current;
    const old = baseRef.current;
    if (r === null || old === null || (s !== "editing" && s !== "rejected")) return;
    const w = optsRef.current.watches;
    const read = (p: string): boolean => w === undefined || w([p]);
    const files: Record<string, string> = {};
    for (const [p, sum] of Object.entries(old.Files ?? {})) {
      if (read(p)) files[p] = sum;
    }
    for (const [p, sum] of Object.entries(r.base.Files ?? {})) {
      if (!read(p)) files[p] = sum;
    }
    baseRef.current = { ...old, Files: files };
  }

  async function send(
    req: Request,
    base: Baseline,
    standalone: boolean,
  ): Promise<SaveResult | null> {
    seqRef.current++;
    lastRef.current = { ...req, standalone };
    setProblems([]);
    setError("");
    setResult(null);
    go("checking");
    let res: SaveResult;
    try {
      res = await client.saveSetting({ kind: req.kind, name: req.name, input: req.input, base });
    } catch (e) {
      if (!aliveRef.current) return null;
      setError(toDeskError(e).code);
      go("editing");
      return null;
    }
    if (!aliveRef.current) return res;
    switch (res.Outcome) {
      case "saved":
        baseRef.current = res.Base;
        currentRef.current = null;
        setResult(res);
        if (standalone) {
          go("viewing");
          await refresh(true);
        } else {
          valueRef.current = draftRef.current;
          setValueView(draftRef.current);
          go("viewing");
        }
        break;
      case "rejected":
        setProblems(res.Problems ?? []);
        setResult(res);
        go("rejected");
        break;
      case "external_change":
        currentRef.current = res.Current;
        setResult(res);
        go("external");
        break;
      case "partial_failure":
        setResult(res);
        go("viewing");
        await refresh(true);
        break;
      default:
        setError("unknown");
        go("editing");
    }
    return res;
  }

  async function save(override?: SaveOverride): Promise<SaveResult | null> {
    const s = stateRef.current;
    if (s !== "viewing" && s !== "editing" && s !== "rejected") return null;
    const o = optsRef.current;
    if (o.ready === false) return null;
    const base = baseRef.current;
    const standalone = override?.input !== undefined;
    if (base === null || (!standalone && draftRef.current === null)) return null;
    const input = override?.input ?? o.serialize(draftRef.current as T);
    return send(
      { kind: override?.kind ?? o.kind, name: override?.name ?? o.name ?? "", input },
      base,
      standalone,
    );
  }

  async function overwrite(): Promise<SaveResult | null> {
    if (stateRef.current !== "external") return null;
    const o = optsRef.current;
    if (o.ready === false) return null;
    const last = lastRef.current;
    let req: Request;
    if (last !== null && last.standalone) req = last;
    else if (draftRef.current === null) return null;
    else
      req = {
        kind: last?.kind ?? o.kind,
        name: last?.name ?? o.name ?? "",
        input: o.serialize(draftRef.current),
      };
    let base = currentRef.current;
    if (base === null) {
      // External came from a home change, not from a save answer: read what the home holds now.
      go("checking");
      seqRef.current++;
      try {
        base = (await o.load()).base;
      } catch (e) {
        if (aliveRef.current) {
          setError(toDeskError(e).code);
          go("external");
        }
        return null;
      }
      if (!aliveRef.current) return null;
    }
    return send(req, base, last !== null && last.standalone);
  }

  async function reload(): Promise<void> {
    const s = stateRef.current;
    if (s === "checking" || s === "writing") return;
    currentRef.current = null;
    setProblems([]);
    setResult(null);
    setError("");
    draftRef.current = valueRef.current;
    setDraftView(valueRef.current);
    go("viewing");
    await refresh(true);
  }

  function discard(): void {
    const s = stateRef.current;
    if (s !== "editing" && s !== "rejected") return;
    draftRef.current = valueRef.current;
    setDraftView(valueRef.current);
    setProblems([]);
    setResult(null);
    setError("");
    go("viewing");
  }

  function setDraft(d: T | ((prev: T) => T)): void {
    const s = stateRef.current;
    if (s === "checking" || s === "writing") return;
    const prev = draftRef.current;
    const next = typeof d === "function" ? (prev === null ? null : (d as (p: T) => T)(prev)) : d;
    if (next === null) return;
    draftRef.current = next;
    setDraftView(next);
    if (s !== "external") {
      setResult(null);
      setProblems([]);
      setError("");
      if (s !== "editing") go("editing");
    }
  }

  const ready = opts.ready !== false;
  useEffect(() => {
    aliveRef.current = true;
    if (ready && stateRef.current === "viewing") void refresh(false);
    return () => {
      aliveRef.current = false;
    };
    // refresh only reads refs and stable setters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready]);

  useEffect(
    () =>
      client.events.onHomeChanged((e) => {
        const s = stateRef.current;
        // The Go side checks again right before it replaces a file.
        if (s === "checking" || s === "writing" || s === "external") return;
        if (s === "viewing") {
          void refresh(false);
          return;
        }
        const w = optsRef.current.watches;
        if (w === undefined || w(e.paths)) {
          currentRef.current = null;
          go("external");
        } else {
          void refreshBase();
        }
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [client],
  );

  const changed =
    value !== null && draft !== null && opts.serialize(draft) !== opts.serialize(value);

  return { state, value, draft, setDraft, save, overwrite, reload, discard, problems, changed, result, error };
}
