export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  ports: number[];
  ip: string | null;
  network: string | null;
  workdir: string | null;
  labels: Record<string, string>;
}

interface DockerPsOutput {
  ID: string;
  Names: string;
  Image: string;
  Ports: string;
  Labels: string;
}

interface DockerInspectOutput {
  NetworkSettings: {
    Networks: Record<
      string,
      {
        IPAddress: string;
      }
    >;
  };
  Config: {
    Labels: Record<string, string>;
    ExposedPorts?: Record<string, unknown>;
    WorkingDir?: string;
  };
}

// Get the compose project from environment variable
function getOwnComposeProject(): string | null {
  return process.env.COMPOSE_PROJECT || null;
}

export async function discoverDockerContainers(): Promise<DockerContainer[]> {
  const containers: DockerContainer[] = [];

  // Check if docker is available
  try {
    const checkProc = Bun.spawn(["docker", "info"], {
      stdout: "pipe",
      stderr: "pipe",
    });
    await checkProc.exited;
    if (checkProc.exitCode !== 0) {
      return [];
    }
  } catch {
    return [];
  }

  // Get our own compose project to filter it out
  const ownProject = getOwnComposeProject();

  // Get running containers
  const psProc = Bun.spawn(
    ["docker", "ps", "--format", "{{json .}}"],
    {
      stdout: "pipe",
      stderr: "pipe",
    }
  );

  const psOutput = await new Response(psProc.stdout).text();
  const psLines = psOutput.split("\n").filter(Boolean);

  for (const line of psLines) {
    try {
      const container: DockerPsOutput = JSON.parse(line);
      const details = await getContainerDetails(container.ID);

      if (details) {
        // Filter out containers from our own compose project
        const containerProject = details.labels["com.docker.compose.project"];
        if (ownProject && containerProject === ownProject) {
          continue;
        }

        containers.push(details);
      }
    } catch {
      // Skip malformed JSON
    }
  }

  return containers;
}

async function getContainerDetails(
  containerId: string
): Promise<DockerContainer | null> {
  const inspectProc = Bun.spawn(["docker", "inspect", containerId], {
    stdout: "pipe",
    stderr: "pipe",
  });

  const output = await new Response(inspectProc.stdout).text();

  try {
    const inspectData: DockerInspectOutput[] = JSON.parse(output);
    const data = inspectData[0];
    if (!data) return null;

    const networks = data.NetworkSettings.Networks;
    const labels = data.Config.Labels || {};

    // Get first available network and IP
    let ip: string | null = null;
    let network: string | null = null;
    for (const [netName, netConfig] of Object.entries(networks)) {
      if (netConfig.IPAddress) {
        ip = netConfig.IPAddress;
        network = netName;
        break;
      }
    }

    // Extract exposed ports
    const ports: number[] = [];
    if (data.Config.ExposedPorts) {
      for (const portSpec of Object.keys(data.Config.ExposedPorts)) {
        const portMatch = portSpec.match(/^(\d+)/);
        if (portMatch?.[1]) {
          ports.push(parseInt(portMatch[1], 10));
        }
      }
    }

    // Get container name from inspect
    const nameProc = Bun.spawn(
      ["docker", "inspect", "--format", "{{.Name}}", containerId],
      {
        stdout: "pipe",
        stderr: "pipe",
      }
    );
    const nameOutput = await new Response(nameProc.stdout).text();
    const name = nameOutput.trim().replace(/^\//, "");

    // Get image name
    const imageProc = Bun.spawn(
      ["docker", "inspect", "--format", "{{.Config.Image}}", containerId],
      {
        stdout: "pipe",
        stderr: "pipe",
      }
    );
    const imageOutput = await new Response(imageProc.stdout).text();
    const image = imageOutput.trim();

    // Get workdir - prefer docker-compose working_dir label, then container's WorkingDir
    const workdir =
      labels["com.docker.compose.project.working_dir"] ||
      data.Config.WorkingDir ||
      null;

    return {
      id: containerId,
      name,
      image,
      ports,
      ip,
      network,
      workdir,
      labels,
    };
  } catch {
    return null;
  }
}

export async function getContainerIp(
  containerIdOrName: string
): Promise<string | null> {
  const proc = Bun.spawn(
    [
      "docker",
      "inspect",
      "--format",
      "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
      containerIdOrName,
    ],
    {
      stdout: "pipe",
      stderr: "pipe",
    }
  );

  const output = await new Response(proc.stdout).text();
  const ip = output.trim();
  return ip || null;
}
