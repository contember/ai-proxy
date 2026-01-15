export interface LocalProcess {
  port: number;
  pid: number;
  command: string;
  args: string;
  workdir: string | null;
}

// Processes to filter out (system/non-dev)
const IGNORED_COMMANDS = new Set([
  "docker-proxy",
  "code",
  "spotify",
  "jetbrains-toolb",
  "chrome",
  "firefox",
  "slack",
  "discord",
  "telegram",
  "signal",
  "zoom",
]);

// Workdirs that indicate container/system processes
const IGNORED_WORKDIRS = new Set(["/", "/app", "/srv", "/root"]);

export async function discoverLocalProcesses(): Promise<LocalProcess[]> {
  // Try ss first (faster), fallback to /proc parsing
  let processes = await tryWithSs();
  if (processes.length === 0) {
    processes = await parseFromProc();
  }

  // Filter out system/non-dev processes
  return processes.filter((p) => {
    if (IGNORED_COMMANDS.has(p.command)) return false;
    if (p.workdir && IGNORED_WORKDIRS.has(p.workdir)) return false;
    return true;
  });
}

// Fast method using ss command
async function tryWithSs(): Promise<LocalProcess[]> {
  const processes: LocalProcess[] = [];

  try {
    const ssProc = Bun.spawn(["ss", "-tlnp"], {
      stdout: "pipe",
      stderr: "pipe",
    });

    const output = await new Response(ssProc.stdout).text();
    const lines = output.split("\n").slice(1);

    for (const line of lines) {
      if (!line.trim()) continue;

      const parts = line.split(/\s+/);
      if (parts.length < 5) continue;

      const localAddr = parts[3];
      if (!localAddr) continue;
      const processInfo = parts.slice(5).join(" ");

      const portMatch = localAddr.match(/:(\d+)$/);
      if (!portMatch?.[1]) continue;
      const port = parseInt(portMatch[1], 10);

      if (port < 1024) continue;

      const pidMatch = processInfo.match(/pid=(\d+)/);
      const cmdMatch = processInfo.match(/\("([^"]+)"/);

      if (!pidMatch?.[1]) continue;

      const pid = parseInt(pidMatch[1], 10);
      const command = cmdMatch?.[1] || "unknown";

      const args = await getProcessArgs(pid);
      const workdir = await getProcessWorkdir(pid);

      processes.push({ port, pid, command, args, workdir });
    }
  } catch {
    // ss not available
  }

  return processes;
}

// Slower fallback parsing /proc/net/tcp
async function parseFromProc(): Promise<LocalProcess[]> {
  const processes: LocalProcess[] = [];
  const seenPorts = new Set<number>();

  // Build inode -> pid map first (much faster than searching per inode)
  const inodeToPid = await buildInodePidMap();

  for (const tcpFile of ["/proc/net/tcp", "/proc/net/tcp6"]) {
    try {
      const content = await Bun.file(tcpFile).text();
      const lines = content.split("\n").slice(1);

      for (const line of lines) {
        const parts = line.trim().split(/\s+/);
        if (parts.length < 10) continue;

        const localAddr = parts[1];
        const state = parts[3];

        if (state !== "0A") continue; // LISTEN

        const portHex = localAddr?.split(":")[1];
        if (!portHex) continue;
        const port = parseInt(portHex, 16);

        if (port < 1024 || seenPorts.has(port)) continue;
        seenPorts.add(port);

        const inode = parts[9];
        if (!inode) continue;

        const pid = inodeToPid.get(inode);
        if (!pid) continue;

        const command = await getProcessCommand(pid);
        const args = await getProcessArgs(pid);
        const workdir = await getProcessWorkdir(pid);

        processes.push({ port, pid, command, args, workdir });
      }
    } catch {
      // File might not exist
    }
  }

  return processes;
}

// Build map of socket inode -> PID (scan once, use many times)
async function buildInodePidMap(): Promise<Map<string, number>> {
  const map = new Map<string, number>();

  try {
    const proc = Bun.spawn(["ls", "/proc"], { stdout: "pipe", stderr: "pipe" });
    const output = await new Response(proc.stdout).text();

    for (const entry of output.split("\n")) {
      if (!/^\d+$/.test(entry)) continue;
      const pid = parseInt(entry, 10);

      try {
        const fdProc = Bun.spawn(["ls", "-l", `/proc/${pid}/fd`], {
          stdout: "pipe",
          stderr: "pipe",
        });
        const fdOutput = await new Response(fdProc.stdout).text();

        // Extract all socket inodes from this process
        const matches = fdOutput.matchAll(/socket:\[(\d+)\]/g);
        for (const match of matches) {
          if (match[1]) map.set(match[1], pid);
        }
      } catch {
        // Can't read this process's fds
      }
    }
  } catch {
    // Error
  }

  return map;
}

async function getProcessCommand(pid: number): Promise<string> {
  try {
    return (await Bun.file(`/proc/${pid}/comm`).text()).trim();
  } catch {
    return "unknown";
  }
}

async function getProcessArgs(pid: number): Promise<string> {
  try {
    const content = await Bun.file(`/proc/${pid}/cmdline`).text();
    return content.split("\0").filter(Boolean).join(" ");
  } catch {
    return "";
  }
}

async function getProcessWorkdir(pid: number): Promise<string | null> {
  try {
    const proc = Bun.spawn(["readlink", `/proc/${pid}/cwd`], {
      stdout: "pipe",
      stderr: "pipe",
    });
    return (await new Response(proc.stdout).text()).trim() || null;
  } catch {
    return null;
  }
}
