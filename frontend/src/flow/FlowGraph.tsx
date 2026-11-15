import { useMemo } from "react";
import {
  Background,
  Handle,
  Position,
  ReactFlow,
} from "@xyflow/react";
import type { Edge, Node, NodeProps } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ComponentView, WorkflowView } from "../api/types";
import { t } from "../i18n";
import { componentTitle } from "./ComponentPanel";

// Layout is computed by hand: one column per step, left to right; the
// components of a step are stacked under its head.
const COL_W = 260;
const NODE_W = 208;
const HEAD_H = 56;
const ROW_H = 64;
const COMP_TOP = 84;
const PAD_Y = 40;

/** Colors come from the theme tokens; the purple is the selection only, the cyan is the wire only. */
const GRAPH_CSS = `
.flow-graph .react-flow {
  --xy-background-color: var(--background);
  --xy-node-color: var(--foreground);
  --xy-node-background-color: var(--card);
  --xy-node-border: 1px solid var(--border);
  --xy-edge-stroke: var(--brand-accent);
  --xy-edge-stroke-width: 1.5;
  --xy-handle-background-color: var(--brand-accent);
  --xy-handle-border-color: var(--background);
  --xy-attribution-background-color: transparent;
  font-family: var(--font-sans);
}
.flow-graph .react-flow__node { font-size: 13px; }
.flow-graph .react-flow__handle { width: 6px; height: 6px; min-width: 0; min-height: 0; }
.flow-graph .react-flow__attribution { color: var(--muted-foreground); }
.flow-graph .react-flow__attribution a { color: var(--muted-foreground); }
`;

interface StepData extends Record<string, unknown> {
  label: string;
  kindLabel: string;
  note: string;
}

interface CompData extends Record<string, unknown> {
  componentId: string;
  title: string;
  source: "bundled" | "home" | "";
  selected: boolean;
  known: boolean;
  onSelect(id: string): void;
}

type StepNodeType = Node<StepData, "step">;
type CompNodeType = Node<CompData, "component">;

function StepNode({ data }: NodeProps<StepNodeType>) {
  return (
    <div
      style={{ width: NODE_W, height: HEAD_H }}
      className="flex flex-col justify-center gap-0.5 rounded-md border border-foreground/15 bg-secondary px-3"
    >
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <span className="truncate font-heading text-sm text-foreground">{data.label}</span>
      <span className="truncate text-xs text-muted-foreground">
        {data.kindLabel}
        {data.note !== "" && ` · ${data.note}`}
      </span>
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}

function ComponentNode({ data }: NodeProps<CompNodeType>) {
  const border = data.selected
    ? "border-primary bg-accent text-accent-foreground"
    : "border-foreground/10 bg-card text-card-foreground hover:bg-muted";
  return (
    <button
      type="button"
      disabled={!data.known}
      aria-pressed={data.known ? data.selected : undefined}
      title={data.title}
      onClick={() => data.onSelect(data.componentId)}
      style={{ width: NODE_W, height: ROW_H - 12 }}
      className={`flex flex-col items-start justify-center gap-0.5 rounded-md border px-3 text-left transition-colors duration-150 motion-reduce:transition-none ${border} ${
        data.known ? "cursor-pointer" : "cursor-default opacity-60"
      }`}
    >
      <span className="flex w-full items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-sm">{data.title}</span>
        {data.source !== "" && (
          <span
            data-source={data.source}
            className={`shrink-0 rounded-sm px-1.5 text-[11px] leading-5 ${
              data.source === "home"
                ? "border border-dashed border-foreground/40 text-foreground"
                : "bg-muted text-muted-foreground"
            }`}
          >
            {t("flow", data.source === "home" ? "sourceHome" : "sourceBundled")}
          </span>
        )}
      </span>
      <span className="w-full truncate font-mono text-[11px] text-muted-foreground">
        {data.componentId}
      </span>
    </button>
  );
}

const NODE_TYPES = { step: StepNode, component: ComponentNode };

function stageLabel(stage: string): string {
  const key = `stage_${stage}`;
  const text = t("flow", key);
  return text === key ? stage : text;
}

function kindOf(kind: string): string {
  if (kind === "optional") return t("flow", "kindOptional");
  if (kind === "gate") return t("flow", "kindGate");
  return t("flow", "kindStage");
}

/**
 * The flow as a diagram, read only: the steps run left to right at equal
 * spacing and the components of each step stand under it. `selectedId` is a
 * component id; only components can be selected.
 */
export function FlowGraph({
  workflow,
  components,
  selectedId,
  onSelect,
}: {
  workflow: WorkflowView;
  components: Map<string, ComponentView>;
  selectedId?: string;
  onSelect(id: string): void;
}) {
  const steps = workflow.workflow.steps;

  const { nodes, edges, height } = useMemo(() => {
    const ns: (StepNodeType | CompNodeType)[] = [];
    const es: Edge[] = [];
    let rows = 0;
    steps.forEach((step, i) => {
      const x = i * COL_W;
      const stepId = `s:${i}`;
      ns.push({
        id: stepId,
        type: "step",
        position: { x, y: 0 },
        data: {
          label: stageLabel(step.stage),
          kindLabel: kindOf(step.kind),
          note: step.after !== "" ? t("flow", "afterStage", { stage: step.after }) : "",
        },
        draggable: false,
        connectable: false,
        selectable: false,
      });
      if (i > 0) {
        es.push({
          id: `e:${i - 1}:${i}`,
          source: `s:${i - 1}`,
          target: stepId,
          selectable: false,
          focusable: false,
        });
      }
      rows = Math.max(rows, step.components.length);
      step.components.forEach((entry, j) => {
        const view = components.get(entry.id);
        ns.push({
          id: `c:${i}:${j}`,
          type: "component",
          position: { x, y: COMP_TOP + j * ROW_H },
          data: {
            componentId: entry.id,
            title: view === undefined ? t("flow", "componentUnknown") : componentTitle(view),
            source:
              view === undefined ? "" : view.component.source === "home" ? "home" : "bundled",
            selected: selectedId === entry.id,
            known: view !== undefined,
            onSelect,
          },
          draggable: false,
          connectable: false,
          selectable: view !== undefined,
        });
      });
    });
    return {
      nodes: ns,
      edges: es,
      height: Math.max(240, COMP_TOP + rows * ROW_H + PAD_Y),
    };
  }, [steps, components, selectedId, onSelect]);

  if (steps.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("flow", "graphEmpty")}</p>;
  }

  return (
    <div
      className="flow-graph overflow-hidden rounded-lg border border-foreground/10"
      style={{ height: Math.min(height, 560) }}
      role="group"
      aria-label={t("flow", "graphLabel", { name: workflow.workflow.name })}
    >
      <style>{GRAPH_CSS}</style>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        nodesDraggable={false}
        nodesConnectable={false}
        edgesFocusable={false}
        elementsSelectable
        fitView
        fitViewOptions={{ padding: 0.12, maxZoom: 1 }}
        minZoom={0.3}
        maxZoom={1.5}
        zoomOnDoubleClick={false}
      >
        <Background gap={16} size={1} color="color-mix(in oklab, var(--foreground) 10%, transparent)" />
      </ReactFlow>
    </div>
  );
}
