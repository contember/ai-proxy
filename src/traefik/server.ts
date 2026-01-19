import { generateTraefikConfig } from "./config-generator";
import { resolveTarget } from "../resolver";
import { setMapping, loadMappings, deleteMapping } from "../config";

const PORT = parseInt(process.env.RESOLVER_PORT || "3001", 10);

interface ResolveResultForHtml {
  type: "process" | "docker";
  target: string;
  port: number;
  reason: string;
}

function getConfirmHtml(hostname: string, result: ResolveResultForHtml): string {
  const targetDesc = result.type === "process"
    ? `localhost:${result.port}`
    : `${result.target}:${result.port} (Docker)`;

  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Confirm routing - ${hostname}</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      margin: 0;
      background: #1a1a2e;
      color: #eee;
    }
    .container {
      text-align: center;
      padding: 2rem;
      max-width: 500px;
    }
    h1 { font-size: 1.5rem; margin-bottom: 1.5rem; color: #6366f1; }
    .mapping {
      background: #2a2a3e;
      border-radius: 12px;
      padding: 1.5rem;
      margin-bottom: 1.5rem;
      text-align: left;
    }
    .mapping-row {
      display: flex;
      justify-content: space-between;
      padding: 0.5rem 0;
      border-bottom: 1px solid #3a3a4e;
    }
    .mapping-row:last-child { border-bottom: none; }
    .label { color: #888; }
    .value { color: #fff; font-family: monospace; }
    .reason {
      margin-top: 1rem;
      padding-top: 1rem;
      border-top: 1px solid #3a3a4e;
      color: #888;
      font-size: 0.9rem;
    }
    .buttons {
      display: flex;
      gap: 1rem;
      justify-content: center;
    }
    button {
      padding: 0.75rem 2rem;
      border: none;
      border-radius: 8px;
      font-size: 1rem;
      cursor: pointer;
      transition: all 0.2s;
    }
    .confirm {
      background: #6366f1;
      color: white;
    }
    .confirm:hover { background: #5558e3; }
    .cancel {
      background: #3a3a4e;
      color: #888;
    }
    .cancel:hover { background: #4a4a5e; color: #fff; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Route ${hostname}?</h1>
    <div class="mapping">
      <div class="mapping-row">
        <span class="label">Hostname</span>
        <span class="value">${hostname}</span>
      </div>
      <div class="mapping-row">
        <span class="label">Type</span>
        <span class="value">${result.type}</span>
      </div>
      <div class="mapping-row">
        <span class="label">Target</span>
        <span class="value">${targetDesc}</span>
      </div>
      <div class="reason">${result.reason}</div>
    </div>
    <div class="buttons">
      <form method="POST" action="/_confirm" style="display:inline">
        <input type="hidden" name="hostname" value="${hostname}">
        <input type="hidden" name="type" value="${result.type}">
        <input type="hidden" name="target" value="${result.target}">
        <input type="hidden" name="port" value="${result.port}">
        <input type="hidden" name="reason" value="${result.reason}">
        <button type="submit" class="confirm">Confirm</button>
      </form>
      <button class="cancel" onclick="window.close()">Cancel</button>
    </div>
  </div>
</body>
</html>`;
}

function getSuccessHtml(hostname: string, result: ResolveResultForHtml): string {
  const targetDesc = result.type === "process"
    ? `localhost:${result.port}`
    : `${result.target}:${result.port} (Docker)`;

  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Routing confirmed - ${hostname}</title>
  <meta http-equiv="refresh" content="3;url=https://${hostname}">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      margin: 0;
      background: #1a1a2e;
      color: #eee;
    }
    .container {
      text-align: center;
      padding: 2rem;
      max-width: 500px;
    }
    .checkmark {
      width: 60px;
      height: 60px;
      border-radius: 50%;
      background: #22c55e;
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 1.5rem;
      font-size: 2rem;
    }
    h1 { font-size: 1.5rem; margin-bottom: 0.5rem; color: #22c55e; }
    .info {
      background: #2a2a3e;
      border-radius: 12px;
      padding: 1rem 1.5rem;
      margin: 1.5rem 0;
      font-family: monospace;
    }
    p { color: #888; }
    a { color: #6366f1; }
  </style>
</head>
<body>
  <div class="container">
    <div class="checkmark">✓</div>
    <h1>Routing Confirmed</h1>
    <div class="info">${hostname} → ${targetDesc}</div>
    <p>Redirecting in 3 seconds...</p>
    <p><a href="https://${hostname}">Go now</a></p>
  </div>
</body>
</html>`;
}

function getErrorHtml(hostname: string, error: string): string {
  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Resolution Failed - ${hostname}</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      margin: 0;
      background: #1a1a2e;
      color: #eee;
    }
    .container {
      text-align: center;
      padding: 2rem;
      max-width: 600px;
    }
    h1 { color: #ef4444; }
    pre {
      background: #2a2a3e;
      padding: 1rem;
      border-radius: 8px;
      text-align: left;
      overflow-x: auto;
    }
    a { color: #6366f1; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Resolution Failed</h1>
    <p>Could not resolve <strong>${hostname}</strong></p>
    <pre>${error}</pre>
    <p><a href="javascript:location.reload()">Try again</a></p>
  </div>
</body>
</html>`;
}

async function handleRequest(req: Request): Promise<Response> {
  const url = new URL(req.url);
  const pathname = url.pathname;

  // Health check
  if (pathname === "/health") {
    return new Response("OK", { status: 200 });
  }

  // Traefik config endpoint
  if (pathname === "/traefik/config") {
    const config = await generateTraefikConfig();
    return new Response(JSON.stringify(config, null, 2), {
      headers: { "Content-Type": "application/json" },
    });
  }

  // Debug endpoint - list all mappings
  if (pathname === "/_debug" || pathname === "/_debug/") {
    const mappings = await loadMappings();
    return new Response(JSON.stringify(mappings, null, 2), {
      headers: { "Content-Type": "application/json" },
    });
  }

  // API: Delete mapping
  if (pathname.startsWith("/_api/mappings/") && req.method === "DELETE") {
    const hostname = pathname.replace("/_api/mappings/", "");
    await deleteMapping(hostname);
    return new Response(JSON.stringify({ deleted: hostname }), {
      headers: { "Content-Type": "application/json" },
    });
  }

  // API: Set mapping manually
  if (pathname.startsWith("/_api/mappings/") && req.method === "PUT") {
    const hostname = pathname.replace("/_api/mappings/", "");
    const body = await req.json();
    await setMapping(hostname, {
      type: body.type,
      target: body.target,
      port: body.port,
      createdAt: new Date().toISOString(),
      llmReason: body.reason || "Manual mapping",
    });
    return new Response(JSON.stringify({ created: hostname }), {
      headers: { "Content-Type": "application/json" },
    });
  }

  // Confirm mapping - user confirmed the LLM suggestion
  if (pathname === "/_confirm" && req.method === "POST") {
    const formData = await req.formData();
    const hostname = formData.get("hostname") as string;
    const type = formData.get("type") as "process" | "docker";
    const target = formData.get("target") as string;
    const port = parseInt(formData.get("port") as string, 10);
    const reason = formData.get("reason") as string;

    if (!hostname || !type || !target || !port) {
      return new Response("Missing required fields", { status: 400 });
    }

    // Save the confirmed mapping
    await setMapping(hostname, {
      type,
      target,
      port,
      createdAt: new Date().toISOString(),
      llmReason: reason,
    });

    console.log(`[Resolver] Confirmed mapping: ${hostname} -> ${type}:${target}:${port}`);

    // Show success page with redirect
    return new Response(getSuccessHtml(hostname, { type, target, port, reason }), {
      status: 200,
      headers: { "Content-Type": "text/html" },
    });
  }

  // Fallback handler - resolve unknown hostname via LLM
  const hostname = req.headers.get("Host");
  if (!hostname) {
    return new Response("Missing Host header", { status: 400 });
  }

  // Strip port from hostname if present
  const cleanHostname = hostname.split(":")[0];

  // Skip resolution for resolver's own hostname
  if (cleanHostname === "resolver" || cleanHostname === "localhost") {
    return new Response("Not found", { status: 404 });
  }

  // Get optional user prompt from query parameter
  const userPrompt = url.searchParams.get("prompt") || undefined;

  console.log(`[Resolver] Resolving hostname: ${cleanHostname}`);

  try {
    // Resolve via LLM (but don't save yet - wait for user confirmation)
    const result = await resolveTarget(cleanHostname, userPrompt);

    console.log(`[Resolver] Resolved ${cleanHostname} -> ${result.type}:${result.target}:${result.port} (awaiting confirmation)`);

    // Show confirmation page - user must confirm before mapping is saved
    return new Response(getConfirmHtml(cleanHostname, result), {
      status: 200,
      headers: { "Content-Type": "text/html" },
    });
  } catch (error) {
    console.error(`[Resolver] Failed to resolve ${cleanHostname}:`, error);

    const errorMessage = error instanceof Error ? error.message : String(error);
    return new Response(getErrorHtml(cleanHostname, errorMessage), {
      status: 500,
      headers: { "Content-Type": "text/html" },
    });
  }
}

console.log(`[Resolver] Starting on port ${PORT}`);

Bun.serve({
  port: PORT,
  fetch: handleRequest,
});

console.log(`[Resolver] Listening on http://localhost:${PORT}`);
console.log(`[Resolver] Traefik config endpoint: http://localhost:${PORT}/traefik/config`);
