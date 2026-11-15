import { useMemo, useSyncExternalStore } from "react";

export type SettingsTab = "agents" | "routing" | "priority" | "workflows";

export type Route =
  | { screen: "flow"; workflowId?: string; componentId?: string }
  | { screen: "skills"; skillKey?: string }
  | { screen: "settings"; tab?: SettingsTab }
  | { screen: "runs" };

const TABS: readonly string[] = ["agents", "routing", "priority", "workflows"];

function decode(s: string): string | undefined {
  if (s === "") return undefined;
  try {
    return decodeURIComponent(s);
  } catch {
    return undefined;
  }
}

/** `#/flow/<id>?component=<id>`, `#/skills/<key>`, `#/settings/routing`, `#/runs`. Anything else is the flow screen. */
export function parseRoute(hash: string): Route {
  let h = hash.startsWith("#") ? hash.slice(1) : hash;
  const q = h.indexOf("?");
  const query = q >= 0 ? h.slice(q + 1) : "";
  if (q >= 0) h = h.slice(0, q);
  const parts = h.split("/").filter((p) => p !== "");
  const rest = parts.length > 1 ? decode(parts.slice(1).join("/")) : undefined;
  switch (parts[0]) {
    case "skills":
      return rest === undefined ? { screen: "skills" } : { screen: "skills", skillKey: rest };
    case "settings":
      return rest !== undefined && TABS.includes(rest)
        ? { screen: "settings", tab: rest as SettingsTab }
        : { screen: "settings" };
    case "runs":
      return { screen: "runs" };
    default: {
      const route: Route = { screen: "flow" };
      if (parts[0] === "flow" && rest !== undefined) route.workflowId = rest;
      // URLSearchParams already decodes the value once; decoding again would break a literal "%".
      const component = new URLSearchParams(query).get("component") ?? "";
      if (parts[0] === "flow" && component !== "") route.componentId = component;
      return route;
    }
  }
}

export function toHash(r: Route): string {
  switch (r.screen) {
    case "flow": {
      let h = "#/flow";
      if (r.workflowId) h += `/${encodeURIComponent(r.workflowId)}`;
      if (r.componentId) h += `?component=${encodeURIComponent(r.componentId)}`;
      return h;
    }
    case "skills":
      return r.skillKey ? `#/skills/${encodeURIComponent(r.skillKey)}` : "#/skills";
    case "settings":
      return r.tab ? `#/settings/${r.tab}` : "#/settings";
    case "runs":
      return "#/runs";
  }
}

function subscribe(cb: () => void): () => void {
  window.addEventListener("hashchange", cb);
  return () => window.removeEventListener("hashchange", cb);
}

function currentHash(): string {
  return window.location.hash;
}

/** The route in the address hash; re-renders when the hash changes. */
export function useRoute(): Route {
  const hash = useSyncExternalStore(subscribe, currentHash, () => "");
  return useMemo(() => parseRoute(hash), [hash]);
}

export function navigate(r: Route): void {
  const hash = toHash(r);
  if (window.location.hash !== hash) window.location.hash = hash;
}
