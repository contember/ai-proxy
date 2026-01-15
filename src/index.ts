import { handleRequest } from "./proxy";

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

const server = Bun.serve({
  port: PORT,
  fetch: handleRequest,
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
