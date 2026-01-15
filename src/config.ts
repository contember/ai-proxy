import { join, dirname } from "path";

export interface RouteMapping {
  type: "process" | "docker";
  target: string; // For process: "localhost", for docker: container name/id
  port: number;
  createdAt: string;
  llmReason?: string;
}

export type Mappings = Record<string, RouteMapping>;

const CONFIG_DIR = join(import.meta.dir, "..", "config");
const MAPPINGS_FILE = join(CONFIG_DIR, "mappings.json");

let mappingsCache: Mappings | null = null;

export async function loadMappings(): Promise<Mappings> {
  if (mappingsCache) {
    return mappingsCache;
  }

  try {
    const file = Bun.file(MAPPINGS_FILE);
    if (await file.exists()) {
      mappingsCache = await file.json();
      return mappingsCache!;
    }
  } catch {
    // File doesn't exist or is invalid
  }

  mappingsCache = {};
  return mappingsCache;
}

export async function saveMappings(mappings: Mappings): Promise<void> {
  // Ensure directory exists
  const dir = dirname(MAPPINGS_FILE);
  await Bun.spawn(["mkdir", "-p", dir]).exited;

  // Write to temp file first, then rename for atomicity
  const tempFile = `${MAPPINGS_FILE}.tmp`;
  await Bun.write(tempFile, JSON.stringify(mappings, null, 2));
  await Bun.spawn(["mv", tempFile, MAPPINGS_FILE]).exited;

  mappingsCache = mappings;
}

export async function getMapping(hostname: string): Promise<RouteMapping | null> {
  const mappings = await loadMappings();
  return mappings[hostname] || null;
}

export async function setMapping(
  hostname: string,
  mapping: RouteMapping
): Promise<void> {
  const mappings = await loadMappings();
  mappings[hostname] = mapping;
  await saveMappings(mappings);
}

export async function deleteMapping(hostname: string): Promise<void> {
  const mappings = await loadMappings();
  delete mappings[hostname];
  await saveMappings(mappings);
}

export function invalidateCache(): void {
  mappingsCache = null;
}
