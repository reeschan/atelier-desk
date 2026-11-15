import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ClientProvider, DeskError, toDeskError, useClient } from "./client";
import type { DeskClient } from "./client";
import { createFakeClient } from "../test/fakeClient";

function Probe({ onClient }: { onClient: (c: DeskClient) => void }) {
  const c = useClient();
  onClient(c);
  return <p>probe</p>;
}

describe("ClientProviderReturnsFake", () => {
  it("useClient returns the fake DeskClient given to ClientProvider", () => {
    const fake = createFakeClient() as unknown as DeskClient;
    let seen: DeskClient | null = null;
    render(
      <ClientProvider client={fake}>
        <Probe
          onClient={(c) => {
            seen = c;
          }}
        />
      </ClientProvider>,
    );
    expect(screen.getByText("probe")).toBeInTheDocument();
    expect(seen).toBe(fake);
  });
});

describe("toDeskError", () => {
  const cases: { name: string; input: unknown; code: string; detail: string }[] = [
    {
      name: "JSON text with a code and an empty detail",
      input: '{"code":"busy_saving","detail":""}',
      code: "busy_saving",
      detail: "",
    },
    {
      name: "JSON text with a code and a detail",
      input: '{"code":"python_unavailable","detail":"no python on PATH"}',
      code: "python_unavailable",
      detail: "no python on PATH",
    },
    {
      name: "JSON text with a code and no detail",
      input: '{"code":"busy_saving"}',
      code: "busy_saving",
      detail: "",
    },
    {
      name: "an unreadable string",
      input: "this is not json",
      code: "unknown",
      detail: "this is not json",
    },
    {
      name: "a Wails timeout message",
      input: "Call to method desk.Service.Versions timed out",
      code: "unknown",
      detail: "Call to method desk.Service.Versions timed out",
    },
    {
      name: "JSON without a code",
      input: '{"detail":"x"}',
      code: "unknown",
      detail: '{"detail":"x"}',
    },
    {
      name: "an Error whose message is Go's JSON",
      input: new Error('{"code":"busy_saving","detail":""}'),
      code: "busy_saving",
      detail: "",
    },
    {
      name: "an Error with a plain message",
      input: new Error("boom"),
      code: "unknown",
      detail: "boom",
    },
  ];

  for (const c of cases) {
    it(`${c.name} becomes code ${c.code}`, () => {
      const e = toDeskError(c.input);
      expect(e).toBeInstanceOf(DeskError);
      expect(e.code).toBe(c.code);
      expect(e.detail).toBe(c.detail);
    });
  }

  it("returns an existing DeskError as it is", () => {
    const original = new DeskError("busy_saving", "");
    expect(toDeskError(original)).toBe(original);
  });
});
