import { handleRequest, handleWebSocketUpgrade } from "./proxy";
import { websocketHandler, type WSData } from "./websocket";

const PORT = parseInt(process.env.PORT || "3000", 10);

console.log(`
╔════════════════════════════════════════════════════╗
║     LLM-Powered Dynamic Reverse Proxy              ║
╠════════════════════════════════════════════════════╣
║  Port: ${PORT.toString().padEnd(43)}║
║  Model: ${(process.env.MODEL || "anthropic/claude-haiku-4.5").padEnd(42)}║
╚════════════════════════════════════════════════════╝
`);

if (!process.env.OPENROUTER_API_KEY) {
  console.warn("⚠ OPENROUTER_API_KEY not set - LLM resolution will fail");
}

const server = Bun.serve<WSData>({
  port: PORT,
  async fetch(req, server) {
    // Check for WebSocket upgrade
    const upgradeHeader = req.headers.get("upgrade");
    if (upgradeHeader?.toLowerCase() === "websocket") {
      const wsData = await handleWebSocketUpgrade(req);
      if (wsData) {
        const success = server.upgrade(req, { data: wsData });
        if (success) {
          return undefined;
        }
        return new Response("WebSocket upgrade failed", { status: 500 });
      }
      return new Response("WebSocket target resolution failed", { status: 502 });
    }

    return handleRequest(req);
  },
  websocket: websocketHandler,
});

console.log(`Server listening on http://localhost:${server.port}`);
console.log(`\nUsage:`);
console.log(`  - Direct: curl http://localhost:${server.port} -H "Host: myapp.localhost"`);
console.log(`  - Via Caddy: curl https://myapp.localhost (with Caddy running)`);
console.log(`\nDebug:`);
console.log(`  - https://proxy.localhost - Debug dashboard`);
console.log(`  - Any URL with /_debug path - JSON debug info`);
console.log(`\nQuery params:`);
console.log(`  - ?force - Force LLM resolution even if mapping exists`);
console.log(`  - ?prompt=xxx - Add custom context for LLM`);
console.log(`\nSecond-level proxy:`);
console.log(`  - /_proxy/api/users - Resolves "api" service relative to current hostname`);
console.log(`  - Example: app.project.localhost/_proxy/api/data -> finds related API service`);
