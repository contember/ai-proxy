import { discoverLocalProcesses, type LocalProcess } from "./discovery/processes";
import { discoverDockerContainers, type DockerContainer } from "./discovery/docker";
import { loadMappings, type Mappings, type RouteMapping } from "./config";

export interface ResolveResult {
  type: "process" | "docker";
  target: string;
  port: number;
  reason: string;
}

interface LLMResponse {
  type: "process" | "docker";
  target: string;
  port: number;
  reason: string;
}

const OPENROUTER_API_URL = "https://openrouter.ai/api/v1/chat/completions";

export async function resolveTarget(
  hostname: string,
  userPrompt?: string
): Promise<ResolveResult> {
  const apiKey = process.env.OPENROUTER_API_KEY;
  if (!apiKey) {
    throw new Error("OPENROUTER_API_KEY is not set");
  }

  const model = process.env.MODEL || "anthropic/claude-haiku-4.5";

  // Gather context
  const [processes, containers, mappings] = await Promise.all([
    discoverLocalProcesses(),
    discoverDockerContainers(),
    loadMappings(),
  ]);

  const prompt = buildPrompt(hostname, processes, containers, mappings, userPrompt);

  const response = await fetch(OPENROUTER_API_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify({
      model,
      messages: [
        {
          role: "system",
          content: getSystemPrompt(),
        },
        {
          role: "user",
          content: prompt,
        },
      ],
      response_format: { type: "json_object" },
    }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`OpenRouter API error: ${response.status} - ${error}`);
  }

  const data = (await response.json()) as {
    choices?: Array<{ message?: { content?: string } }>;
  };
  const content = data.choices?.[0]?.message?.content;

  if (!content) {
    throw new Error("No response from LLM");
  }

  try {
    // Strip markdown code blocks if present
    const jsonContent = content
      .replace(/^```(?:json)?\s*/i, "")
      .replace(/\s*```$/i, "")
      .trim();

    const result: LLMResponse = JSON.parse(jsonContent);
    validateResult(result);
    return result;
  } catch (e) {
    throw new Error(`Failed to parse LLM response: ${content}`);
  }
}

function getSystemPrompt(): string {
  return `You are a routing resolver for a local development proxy. Your job is to determine which local service a request should be forwarded to based on the hostname.

You will receive:
1. The hostname from the request (e.g., "myapp.localhost", "api.project.localhost")
2. A list of locally running processes with their ports, commands, arguments, and working directories
3. A list of Docker containers with their names, images, exposed ports, IP addresses, and working directories
4. Current routing mappings for context

Your task is to analyze the hostname and determine the best matching service. Consider:
- Hostname patterns (e.g., "vite.myproject.localhost" might match a Vite process running in a "myproject" directory)
- Service types (e.g., a hostname containing "api" might route to a backend service)
- Project names in the hostname vs working directories
- Container names vs hostname parts

Respond with a JSON object:
{
  "type": "process" | "docker",
  "target": "localhost" for process, or container name for docker,
  "port": the port number to connect to,
  "reason": "brief explanation of why this target was chosen"
}

If no suitable target is found, still provide your best guess with explanation.`;
}

function buildPrompt(
  hostname: string,
  processes: LocalProcess[],
  containers: DockerContainer[],
  mappings: Mappings,
  userPrompt?: string
): string {
  let prompt = `Hostname to resolve: ${hostname}\n\n`;

  prompt += "## Local Processes\n";
  if (processes.length === 0) {
    prompt += "No local processes with open ports found.\n";
  } else {
    for (const proc of processes) {
      prompt += `- Port ${proc.port}: ${proc.command}`;
      if (proc.args) prompt += ` (args: ${proc.args})`;
      if (proc.workdir) prompt += ` [workdir: ${proc.workdir}]`;
      prompt += "\n";
    }
  }

  prompt += "\n## Docker Containers\n";
  if (containers.length === 0) {
    prompt += "No Docker containers found.\n";
  } else {
    for (const container of containers) {
      prompt += `- ${container.name} (image: ${container.image})`;
      if (container.ports.length > 0) prompt += ` ports: ${container.ports.join(", ")}`;
      if (container.ip) prompt += ` [ip: ${container.ip}]`;
      if (container.workdir) prompt += ` [workdir: ${container.workdir}]`;
      prompt += "\n";
    }
  }

  prompt += "\n## Current Mappings\n";
  const mappingEntries = Object.entries(mappings);
  if (mappingEntries.length === 0) {
    prompt += "No existing mappings.\n";
  } else {
    for (const [host, mapping] of mappingEntries) {
      prompt += `- ${host} -> ${mapping.type}:${mapping.target}:${mapping.port}`;
      if (mapping.llmReason) prompt += ` (${mapping.llmReason})`;
      prompt += "\n";
    }
  }

  if (userPrompt) {
    prompt += `\n## Additional Context from User\n${userPrompt}\n`;
  }

  return prompt;
}

function validateResult(result: unknown): asserts result is LLMResponse {
  if (typeof result !== "object" || result === null) {
    throw new Error("Result must be an object");
  }

  const r = result as Record<string, unknown>;

  if (r.type !== "process" && r.type !== "docker") {
    throw new Error('type must be "process" or "docker"');
  }

  if (typeof r.target !== "string" || !r.target) {
    throw new Error("target must be a non-empty string");
  }

  if (typeof r.port !== "number" || r.port < 1 || r.port > 65535) {
    throw new Error("port must be a valid port number");
  }

  if (typeof r.reason !== "string") {
    throw new Error("reason must be a string");
  }
}
