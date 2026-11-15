import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});

// jsdom lacks these browser APIs; @xyflow/react needs them. Simple stand-ins.
if (typeof window !== "undefined") {
  if (typeof window.matchMedia !== "function") {
    window.matchMedia = (query: string): MediaQueryList =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }) as MediaQueryList;
  }

  if (typeof globalThis.ResizeObserver === "undefined") {
    class FakeResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
    globalThis.ResizeObserver = FakeResizeObserver as unknown as typeof ResizeObserver;
  }

  if (typeof globalThis.DOMMatrixReadOnly === "undefined") {
    class FakeDOMMatrixReadOnly {
      m22 = 1;
      constructor(_transform?: string) {}
    }
    globalThis.DOMMatrixReadOnly = FakeDOMMatrixReadOnly as unknown as typeof DOMMatrixReadOnly;
  }
}
