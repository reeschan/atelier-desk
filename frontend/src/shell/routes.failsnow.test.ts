import { describe, expect, it } from "vitest";
import { parseRoute, toHash } from "./routes";
import type { Route } from "./routes";

// Fails now: parseRoute decodes the `component` query twice (routes.ts:42), so a
// componentId with "%" does not survive toHash -> parseRoute.
describe("routes (fails now)", () => {
  it("ComponentIdWithPercentRoundTrips", () => {
    for (const componentId of ["plain", "a%b", "a%41", "50%"]) {
      const r: Route = { screen: "flow", workflowId: "wf", componentId };
      expect(parseRoute(toHash(r))).toEqual(r);
    }
  });
});
