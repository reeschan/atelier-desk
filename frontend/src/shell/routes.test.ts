import { describe, expect, it } from "vitest";
import { parseRoute, toHash } from "./routes";
import type { Route } from "./routes";

describe("routes", () => {
  it("ParseRouteToHashRoundTrip: toHash then parseRoute gives the same Route", () => {
    const routes: Route[] = [
      { screen: "flow" },
      { screen: "flow", workflowId: "wf" },
      { screen: "flow", workflowId: "my flow/１", componentId: "comp-1" },
      { screen: "flow", componentId: "plain" },
      { screen: "skills" },
      { screen: "skills", skillKey: "studio/impl" },
      { screen: "settings" },
      { screen: "settings", tab: "agents" },
      { screen: "settings", tab: "routing" },
      { screen: "settings", tab: "priority" },
      { screen: "settings", tab: "workflows" },
      { screen: "runs" },
    ];
    for (const r of routes) {
      expect(parseRoute(toHash(r))).toEqual(r);
    }
  });

  it("ParseRouteToHashRoundTrip: an unknown hash is the flow screen", () => {
    expect(parseRoute("")).toEqual({ screen: "flow" });
    expect(parseRoute("#/nowhere/x")).toEqual({ screen: "flow" });
  });
});
