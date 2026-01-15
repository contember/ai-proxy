import { discoverLocalProcesses } from "./discovery/processes";
import { discoverDockerContainers } from "./discovery/docker";
import { loadMappings } from "./config";

export async function handleDebugRequest(): Promise<Response> {
  const [processes, containers, mappings] = await Promise.all([
    discoverLocalProcesses().catch((e) => ({ error: String(e) })),
    discoverDockerContainers().catch((e) => ({ error: String(e) })),
    loadMappings().catch((e) => ({ error: String(e) })),
  ]);

  const debug = {
    timestamp: new Date().toISOString(),
    localProcesses: processes,
    dockerContainers: containers,
    mappings: mappings,
    env: {
      PORT: process.env.PORT || "3000",
      MODEL: process.env.MODEL || "anthropic/claude-haiku-4.5",
      OPENROUTER_API_KEY: process.env.OPENROUTER_API_KEY ? "[set]" : "[not set]",
    },
  };

  return new Response(JSON.stringify(debug, null, 2), {
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export function renderDebugHtml(debug: unknown): string {
  return `<!DOCTYPE html>
<html>
<head>
  <title>LLM Proxy Debug</title>
  <style>
    * { box-sizing: border-box; }
    body {
      font-family: system-ui, -apple-system, sans-serif;
      margin: 0; padding: 20px;
      background: #0a0a0a; color: #e0e0e0;
    }
    h1 { color: #fff; margin-bottom: 5px; }
    h2 { color: #888; font-size: 14px; font-weight: normal; margin-top: 0; }
    h3 { color: #10b981; margin-top: 30px; border-bottom: 1px solid #333; padding-bottom: 8px; }
    pre {
      background: #1a1a1a; padding: 15px; border-radius: 8px;
      overflow-x: auto; font-size: 13px; line-height: 1.5;
    }
    .section { margin-bottom: 30px; }
    .badge {
      display: inline-block; padding: 2px 8px; border-radius: 4px;
      font-size: 12px; margin-right: 8px;
    }
    .badge-process { background: #3b82f6; }
    .badge-docker { background: #8b5cf6; }
    .badge-port { background: #333; color: #10b981; }
    table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 10px; border-bottom: 1px solid #333; }
    th { color: #888; font-weight: normal; font-size: 12px; text-transform: uppercase; }
    .mono { font-family: monospace; }
    .dim { color: #666; }
    a { color: #10b981; }
    .refresh {
      position: fixed; top: 20px; right: 20px;
      background: #10b981; color: #000; border: none;
      padding: 8px 16px; border-radius: 6px; cursor: pointer;
    }
    .refresh:hover { background: #059669; }
    .btn {
      padding: 4px 10px; border: none; border-radius: 4px;
      cursor: pointer; font-size: 12px; margin-right: 5px;
    }
    .btn-edit { background: #3b82f6; color: #fff; }
    .btn-edit:hover { background: #2563eb; }
    .btn-delete { background: #ef4444; color: #fff; }
    .btn-delete:hover { background: #dc2626; }
    .btn-add { background: #10b981; color: #000; }
    .btn-add:hover { background: #059669; }
    .actions { white-space: nowrap; }
    h3 .btn { vertical-align: middle; margin-left: 10px; }
  </style>
</head>
<body>
  <button class="refresh" onclick="location.reload()">Refresh</button>
  <h1>LLM Proxy Debug</h1>
  <h2 id="timestamp"></h2>

  <div class="section">
    <h3>Environment</h3>
    <pre id="env"></pre>
  </div>

  <div class="section">
    <h3>Local Processes</h3>
    <div id="processes"></div>
  </div>

  <div class="section">
    <h3>Docker Containers</h3>
    <div id="containers"></div>
  </div>

  <div class="section">
    <h3>Mappings <button class="btn btn-add" onclick="addMapping()">+ Add</button></h3>
    <div id="mappings"></div>
  </div>

  <div class="section">
    <h3>Raw JSON</h3>
    <pre id="raw"></pre>
  </div>

  <script>
    const data = ${JSON.stringify(debug)};

    document.getElementById('timestamp').textContent = new Date(data.timestamp).toLocaleString();
    document.getElementById('env').textContent = JSON.stringify(data.env, null, 2);
    document.getElementById('raw').textContent = JSON.stringify(data, null, 2);

    // Render processes
    const processesEl = document.getElementById('processes');
    if (Array.isArray(data.localProcesses) && data.localProcesses.length > 0) {
      processesEl.innerHTML = '<table><thead><tr><th>Port</th><th>Command</th><th>Working Directory</th></tr></thead><tbody>' +
        data.localProcesses.map(p =>
          '<tr>' +
          '<td><span class="badge badge-port">' + p.port + '</span></td>' +
          '<td class="mono"><strong>' + escapeHtml(p.command) + '</strong> <span class="dim" title="' + escapeHtml(p.args || '') + '">' + formatArgs(p.args) + '</span></td>' +
          '<td class="mono dim">' + escapeHtml(p.workdir || '-') + '</td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>';
    } else {
      processesEl.innerHTML = '<p class="dim">No local processes found</p>';
    }

    function formatArgs(args) {
      if (!args) return '';
      // Show last meaningful parts of args (skip long paths at start)
      const parts = args.split(' ');
      // Find script/entry point (usually .js, .ts, or after node/bun/python)
      const interesting = parts.filter(p =>
        p.endsWith('.js') || p.endsWith('.ts') || p.endsWith('.py') ||
        p.startsWith('--') || p.startsWith('-p')
      ).slice(0, 5);
      return interesting.length > 0 ? interesting.join(' ') : args.substring(0, 80);
    }

    // Render containers
    const containersEl = document.getElementById('containers');
    if (Array.isArray(data.dockerContainers) && data.dockerContainers.length > 0) {
      containersEl.innerHTML = '<table><thead><tr><th>Name</th><th>Image</th><th>Ports</th><th>IP</th><th>Working Directory</th></tr></thead><tbody>' +
        data.dockerContainers.map(c =>
          '<tr>' +
          '<td class="mono">' + escapeHtml(c.name) + '</td>' +
          '<td class="mono dim">' + escapeHtml(c.image) + '</td>' +
          '<td>' + (c.ports || []).map(p => '<span class="badge badge-port">' + p + '</span>').join('') + '</td>' +
          '<td class="mono">' + escapeHtml(c.ip || '-') + '</td>' +
          '<td class="mono dim">' + escapeHtml(c.workdir || '-') + '</td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>';
    } else {
      containersEl.innerHTML = '<p class="dim">No Docker containers found</p>';
    }

    // Render mappings
    const mappingsEl = document.getElementById('mappings');
    const mappingEntries = Object.entries(data.mappings || {});
    if (mappingEntries.length > 0) {
      mappingsEl.innerHTML = '<table><thead><tr><th>Hostname</th><th>Target</th><th>Reason</th><th>Actions</th></tr></thead><tbody>' +
        mappingEntries.map(([host, m]) =>
          '<tr id="row-' + escapeHtml(host).replace(/\\./g, '-') + '">' +
          '<td><a href="https://' + escapeHtml(host) + '">' + escapeHtml(host) + '</a></td>' +
          '<td><span class="badge badge-' + m.type + '">' + m.type + '</span><span class="mono">' + escapeHtml(m.target) + ':' + m.port + '</span></td>' +
          '<td class="dim">' + escapeHtml(m.llmReason || '-') + '</td>' +
          '<td class="actions">' +
            '<button class="btn btn-edit" onclick="editMapping(\\'' + escapeHtml(host) + '\\', \\'' + m.type + '\\', \\'' + escapeHtml(m.target) + '\\', ' + m.port + ')">Edit</button>' +
            '<button class="btn btn-delete" onclick="deleteMapping(\\'' + escapeHtml(host) + '\\')">Delete</button>' +
          '</td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>';
    } else {
      mappingsEl.innerHTML = '<p class="dim">No mappings yet</p>';
    }

    async function deleteMapping(hostname) {
      if (!confirm('Delete mapping for ' + hostname + '?')) return;
      const res = await fetch('/_api/mappings/' + encodeURIComponent(hostname), { method: 'DELETE' });
      if (res.ok) location.reload();
      else alert('Failed to delete: ' + await res.text());
    }

    function editMapping(hostname, type, target, port) {
      const newType = prompt('Type (process/docker):', type);
      if (!newType) return;
      const newTarget = prompt('Target:', target);
      if (!newTarget) return;
      const newPort = prompt('Port:', port);
      if (!newPort) return;

      fetch('/_api/mappings/' + encodeURIComponent(hostname), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type: newType, target: newTarget, port: parseInt(newPort, 10) })
      }).then(res => {
        if (res.ok) location.reload();
        else res.text().then(t => alert('Failed: ' + t));
      });
    }

    function addMapping() {
      const hostname = prompt('Hostname (e.g., app.project.localhost):');
      if (!hostname) return;
      const type = prompt('Type (process/docker):', 'process');
      if (!type) return;
      const target = prompt('Target (localhost for process, container name for docker):', type === 'process' ? 'localhost' : '');
      if (!target) return;
      const port = prompt('Port:');
      if (!port) return;

      fetch('/_api/mappings/' + encodeURIComponent(hostname), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, target, port: parseInt(port, 10) })
      }).then(res => {
        if (res.ok) location.reload();
        else res.text().then(t => alert('Failed: ' + t));
      });
    }

    function escapeHtml(str) {
      if (!str) return '';
      return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/'/g, '&#39;');
    }
  </script>
</body>
</html>`;
}

export async function handleDebugHtmlRequest(): Promise<Response> {
  const [processes, containers, mappings] = await Promise.all([
    discoverLocalProcesses().catch((e) => ({ error: String(e) })),
    discoverDockerContainers().catch((e) => ({ error: String(e) })),
    loadMappings().catch((e) => ({ error: String(e) })),
  ]);

  const debug = {
    timestamp: new Date().toISOString(),
    localProcesses: processes,
    dockerContainers: containers,
    mappings: mappings,
    env: {
      PORT: process.env.PORT || "3000",
      MODEL: process.env.MODEL || "anthropic/claude-haiku-4.5",
      OPENROUTER_API_KEY: process.env.OPENROUTER_API_KEY ? "[set]" : "[not set]",
    },
  };

  return new Response(renderDebugHtml(debug), {
    headers: {
      "Content-Type": "text/html; charset=utf-8",
    },
  });
}
