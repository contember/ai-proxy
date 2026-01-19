import { loadMappings, type RouteMapping } from "../config";
import { getContainerIp } from "../discovery/docker";

export interface TraefikConfig {
  http: {
    routers: Record<string, TraefikRouter>;
    services: Record<string, TraefikService>;
  };
}

interface TraefikRouter {
  rule: string;
  service: string;
  priority?: number;
  tls?: Record<string, never>;
}

interface TraefikService {
  loadBalancer: {
    servers: Array<{ url: string }>;
  };
}

function hostnameToServiceName(hostname: string): string {
  // Convert hostname to valid Traefik service name
  // e.g., "app.project.localhost" -> "app-project-localhost"
  // Also handle colons from related service mappings
  return hostname.replace(/[.:]/g, "-");
}

function isValidHostname(hostname: string): boolean {
  // Filter out related service mappings (contain ":")
  // These are internal mappings like "app.localhost:api"
  return !hostname.includes(":");
}

export async function generateTraefikConfig(): Promise<TraefikConfig> {
  const mappings = await loadMappings();

  const routers: Record<string, TraefikRouter> = {};
  const services: Record<string, TraefikService> = {};

  // Add route for each known mapping
  for (const [hostname, mapping] of Object.entries(mappings)) {
    // Skip related service mappings - they're not real hostnames
    if (!isValidHostname(hostname)) {
      continue;
    }

    const serviceName = hostnameToServiceName(hostname);
    const targetUrl = await buildTargetUrl(mapping);

    routers[serviceName] = {
      rule: `Host(\`${hostname}\`)`,
      service: serviceName,
      tls: {},
    };

    services[serviceName] = {
      loadBalancer: {
        servers: [{ url: targetUrl }],
      },
    };
  }

  // Add fallback route for unknown *.localhost hostnames
  // Priority 1 ensures it's only matched if no other route matches
  // Traefik v3 syntax for HostRegexp
  routers["fallback-resolver"] = {
    rule: "HostRegexp(`.+\\.localhost`)",
    service: "fallback-resolver",
    priority: 1,
    tls: {},
  };

  const resolverHost = process.env.RESOLVER_HOST || "127.0.0.1";
  const resolverPort = process.env.RESOLVER_PORT || "3001";

  services["fallback-resolver"] = {
    loadBalancer: {
      servers: [{ url: `http://${resolverHost}:${resolverPort}` }],
    },
  };

  return {
    http: {
      routers,
      services,
    },
  };
}

async function buildTargetUrl(mapping: RouteMapping): Promise<string> {
  if (mapping.type === "process") {
    return `http://127.0.0.1:${mapping.port}`;
  }

  // Docker container - get IP
  const ip = await getContainerIp(mapping.target);
  if (!ip) {
    // Fallback to container name (works if on same Docker network)
    return `http://${mapping.target}:${mapping.port}`;
  }

  return `http://${ip}:${mapping.port}`;
}
