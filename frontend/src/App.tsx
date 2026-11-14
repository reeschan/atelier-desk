import { useMemo } from "react";
import { ClientProvider, createWailsClient } from "./api/client";
import type { DeskClient } from "./api/client";
import { Shell } from "./shell/Shell";

/** `client` is the Go side; a test hands in a fake one. */
export default function App({ client }: { client?: DeskClient }) {
  const real = useMemo(() => client ?? createWailsClient(), [client]);
  return (
    <ClientProvider client={real}>
      <Shell />
    </ClientProvider>
  );
}
