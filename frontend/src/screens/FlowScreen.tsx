import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DeskError, toDeskError, useClient } from "../api/client";
import type { ComponentView, WorkflowView } from "../api/types";
import { ComponentPanel } from "../flow/ComponentPanel";
import { FlowGraph } from "../flow/FlowGraph";
import { errorText, t } from "../i18n";
import { navigate, useRoute } from "../shell/routes";

interface Loaded {
  workflows: WorkflowView[];
  components: Map<string, ComponentView>;
}

type Load =
  | { state: "loading" }
  | { state: "failed"; error: DeskError }
  | ({ state: "ready" } & Loaded);

function SourceMark({ source, bundled }: { source: string; bundled: boolean }) {
  const home = source === "home";
  return (
    <span className="flex items-center gap-1.5">
      <span
        data-source={home ? "home" : "bundled"}
        className={`rounded-sm px-1.5 text-[11px] leading-5 ${
          home
            ? "border border-dashed border-foreground/40 text-foreground"
            : "bg-muted text-muted-foreground"
        }`}
      >
        {t("flow", home ? "sourceHome" : "sourceBundled")}
      </span>
      {home && bundled && (
        <span className="text-[11px] text-muted-foreground">{t("flow", "homeOverridesBundled")}</span>
      )}
    </span>
  );
}

/** The flow screen: the flows on the left, the diagram of the chosen one and the detail panel under it. */
export function FlowScreen() {
  const client = useClient();
  const route = useRoute();
  const [load, setLoad] = useState<Load>({ state: "loading" });

  // Counts reads; a read whose number is no longer current (a newer read started, or the screen
  // unmounted) drops its answer.
  const seq = useRef(0);
  const loadState = useRef<Load["state"]>("loading");
  loadState.current = load.state;

  const read = useCallback(async (): Promise<void> => {
    const mine = ++seq.current;
    try {
      const [workflows, list] = await Promise.all([client.workflows(), client.components()]);
      if (mine !== seq.current) return;
      const components = new Map<string, ComponentView>();
      for (const c of list) components.set(c.component.id, c);
      setLoad({ state: "ready", workflows, components });
    } catch (e) {
      if (mine !== seq.current) return;
      setLoad({ state: "failed", error: toDeskError(e) });
    }
  }, [client]);

  useEffect(() => {
    let live = true;
    let seen: { state: string; version: string } | undefined;
    const guarded = () => {
      if (live) void read();
    };
    // Read again when the connection becomes ready or the version changes; never keep showing
    // the flows of the previous version while the new ones are read.
    const onConnection = (c: { state: string; version: string }) => {
      if (!live) return;
      const prev = seen;
      seen = { state: c.state, version: c.version };
      const versionChanged = prev === undefined || prev.version !== c.version;
      if (c.state === "checking") {
        if (versionChanged) {
          seq.current++;
          setLoad({ state: "loading" });
        }
        return;
      }
      const stateChanged = prev === undefined || prev.state !== c.state;
      const needRead =
        versionChanged || stateChanged || (c.state === "ready" && loadState.current !== "ready");
      if (!needRead) return;
      if (versionChanged) setLoad({ state: "loading" });
      void read();
    };
    guarded();
    const offHome = client.events.onHomeChanged(guarded);
    const offConnection = client.events.onConnectionChanged(onConnection);
    return () => {
      live = false;
      seq.current++;
      offHome();
      offConnection();
    };
  }, [client, read]);

  const workflowId = route.screen === "flow" ? route.workflowId : undefined;
  const componentId = route.screen === "flow" ? route.componentId : undefined;

  const current = useMemo(() => {
    if (load.state !== "ready") return undefined;
    if (workflowId === undefined) return load.workflows[0];
    return load.workflows.find((w) => w.workflow.name === workflowId);
  }, [load, workflowId]);

  const onSelect = useCallback(
    (id: string) => {
      if (current === undefined) return;
      navigate({ screen: "flow", workflowId: current.workflow.name, componentId: id });
    },
    [current],
  );

  if (load.state === "loading") {
    return <p className="text-sm text-muted-foreground">{t("flow", "loading")}</p>;
  }

  if (load.state === "failed") {
    return (
      <div role="alert" className="flex flex-col gap-3">
        <p className="text-sm text-destructive">
          {t("flow", "loadFailed")}: {errorText(load.error.code)}
        </p>
        {load.error.detail !== "" && (
          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted-foreground">{t("flow", "diagnosticLabel")}</span>
            <pre className="overflow-auto rounded-md border border-foreground/10 bg-card p-3 font-mono text-xs whitespace-pre-wrap">
              {load.error.detail}
            </pre>
          </div>
        )}
      </div>
    );
  }

  const selected = componentId === undefined ? undefined : load.components.get(componentId);

  return (
    <div className="flex gap-6">
      <nav aria-label={t("flow", "listLabel")} className="flex w-56 shrink-0 flex-col gap-1">
        <span className="mb-1 text-xs text-muted-foreground">
          {t("flow", "listCount", { count: load.workflows.length })}
        </span>
        {load.workflows.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("flow", "listEmpty")}</p>
        )}
        {load.workflows.map((w) => {
          const active = current !== undefined && current.workflow.name === w.workflow.name;
          return (
            <a
              key={w.workflow.name}
              href={`#/flow/${encodeURIComponent(w.workflow.name)}`}
              aria-current={active ? "page" : undefined}
              className={`flex flex-col gap-1 rounded-md px-3 py-2 transition-colors duration-150 motion-reduce:transition-none ${
                active ? "bg-card text-foreground" : "text-muted-foreground hover:bg-card/60 hover:text-foreground"
              }`}
            >
              <span className="font-mono text-sm">{w.workflow.name}</span>
              <SourceMark source={w.workflow.source} bundled={w.workflow.bundled} />
            </a>
          );
        })}
      </nav>

      <div className="flex min-w-0 flex-1 flex-col gap-4">
        {current === undefined ? (
          <p className="text-sm text-muted-foreground">
            {workflowId === undefined
              ? t("flow", "selectWorkflow")
              : t("flow", "notFoundWorkflow", { name: workflowId })}
          </p>
        ) : (
          <>
            <header className="flex flex-wrap items-center gap-3">
              <h1 className="font-heading text-lg text-foreground">{current.workflow.name}</h1>
              <SourceMark source={current.workflow.source} bundled={current.workflow.bundled} />
              {current.workflow.description.ja !== "" && (
                <p className="w-full text-sm text-muted-foreground">{current.workflow.description.ja}</p>
              )}
            </header>
            <FlowGraph
              workflow={current}
              components={load.components}
              selectedId={componentId}
              onSelect={onSelect}
            />
            {selected !== undefined && <ComponentPanel key={selected.component.id} view={selected} />}
          </>
        )}
      </div>
    </div>
  );
}
