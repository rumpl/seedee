import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CIService } from "@/gen/seedee/v1/seedee_pb";

/**
 * ConnectRPC transport configured to talk to the seedee backend.
 * In development Vite proxies `/seedee.v1` to http://localhost:8080.
 */
const transport = createConnectTransport({
  baseUrl: "/",
});

/**
 * Typed ConnectRPC client for the seedee CIService.
 * All RPC methods are available with full type safety.
 */
export const ciClient = createClient(CIService, transport);
