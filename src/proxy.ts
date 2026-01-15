import { getMapping, setMapping, deleteMapping, type RouteMapping } from "./config";
import { resolveTarget } from "./resolver";
import { getContainerIp } from "./discovery/docker";
import { handleDebugRequest, handleDebugHtmlRequest } from "./debug";

export async function handleRequest(req: Request): Promise<Response> {
  const url = new URL(req.url);
  const hostname = extractHostname(req);

  console.log(`[Request] ${req.method} ${hostname}${url.pathname}`);

  if (!hostname) {
    return new Response("Missing Host header", { status: 400 });
  }

  // Caddy on_demand TLS check - approve all .localhost domains
  if (url.pathname === "/_caddy/check") {
    const domain = url.searchParams.get("domain") || hostname;
    if (domain?.endsWith(".localhost")) {
      return new Response("OK", { status: 200 });
    }
    return new Response("Not allowed", { status: 403 });
  }

  // Debug routes: proxy.localhost or /_debug path
  if (hostname === "proxy.localhost" || url.pathname.startsWith("/_debug")) {
    const acceptsHtml = req.headers.get("accept")?.includes("text/html");
    if (acceptsHtml || url.pathname.startsWith("/_debug")) {
      return handleDebugHtmlRequest();
    }
    return handleDebugRequest();
  }

  // API routes for managing mappings
  if (url.pathname.startsWith("/_api/mappings/")) {
    const mappingHostname = decodeURIComponent(url.pathname.replace("/_api/mappings/", ""));

    if (req.method === "DELETE") {
      await deleteMapping(mappingHostname);
      return new Response("Deleted", { status: 200 });
    }

    if (req.method === "PUT") {
      try {
        const body = await req.json() as { type: string; target: string; port: number };
        if (body.type !== "process" && body.type !== "docker") {
          return new Response("Invalid type", { status: 400 });
        }
        const mapping: RouteMapping = {
          type: body.type,
          target: body.target,
          port: body.port,
          createdAt: new Date().toISOString(),
          llmReason: "Manually edited",
        };
        await setMapping(mappingHostname, mapping);
        return new Response("Updated", { status: 200 });
      } catch (e) {
        return new Response("Invalid JSON", { status: 400 });
      }
    }

    return new Response("Method not allowed", { status: 405 });
  }

  // Ignore common browser requests that shouldn't trigger LLM
  if (url.pathname === "/favicon.ico" || url.pathname === "/robots.txt") {
    return new Response(null, { status: 404 });
  }

  // Check for special query params
  const force = url.searchParams.has("force");
  const userPrompt = url.searchParams.get("prompt") || undefined;

  // Remove our special params from the URL before proxying
  url.searchParams.delete("force");
  url.searchParams.delete("prompt");

  try {
    // Get or resolve target
    let mapping = await getMapping(hostname);

    if (!mapping || force) {
      console.log(`[Resolver] Resolving target for: ${hostname}${force ? " (forced)" : ""}`);

      const result = await resolveTarget(hostname, userPrompt);
      console.log(`[Resolver] Result: ${result.type}:${result.target}:${result.port} - ${result.reason}`);

      mapping = {
        type: result.type,
        target: result.target,
        port: result.port,
        createdAt: new Date().toISOString(),
        llmReason: result.reason,
      };

      await setMapping(hostname, mapping);
    }

    // Determine final target URL
    const targetUrl = await buildTargetUrl(mapping, url);
    console.log(`[Proxy] ${hostname} -> ${targetUrl}`);

    // Proxy the request
    return await proxyRequest(req, targetUrl);
  } catch (error) {
    console.error(`[Error] ${error}`);
    return new Response(`Proxy error: ${error}`, { status: 502 });
  }
}

function extractHostname(req: Request): string | null {
  const host = req.headers.get("host");
  if (!host) return null;

  // Remove port if present
  return host.split(":")[0] ?? null;
}

async function buildTargetUrl(mapping: RouteMapping, originalUrl: URL): Promise<string> {
  let host: string;

  if (mapping.type === "process") {
    host = "127.0.0.1";
  } else {
    // Docker container - get IP
    const ip = await getContainerIp(mapping.target);
    if (!ip) {
      throw new Error(`Cannot resolve IP for container: ${mapping.target}`);
    }
    host = ip;
  }

  // Build target URL preserving path and query
  const targetUrl = new URL(originalUrl.pathname + originalUrl.search, `http://${host}:${mapping.port}`);
  return targetUrl.toString();
}

async function proxyRequest(originalReq: Request, targetUrl: string): Promise<Response> {
  // Clone headers, but modify some
  const headers = new Headers(originalReq.headers);

  // Remove headers that shouldn't be forwarded
  headers.delete("host");
  headers.delete("connection");

  // Create the proxied request
  const proxyReq = new Request(targetUrl, {
    method: originalReq.method,
    headers,
    body: originalReq.body,
    redirect: "manual", // Don't follow redirects automatically
  });

  try {
    const response = await fetch(proxyReq);

    // Clone response headers
    const responseHeaders = new Headers(response.headers);

    // Return the proxied response
    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });
  } catch (error) {
    throw new Error(`Failed to connect to target: ${error}`);
  }
}
