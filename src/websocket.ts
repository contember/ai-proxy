import type { ServerWebSocket } from "bun";

interface WSData {
  targetUrl: string;
  targetWs: WebSocket | null;
}

export const websocketHandler = {
  open(ws: ServerWebSocket<WSData>) {
    const { targetUrl } = ws.data;
    console.log(`[WebSocket] Connecting to ${targetUrl}`);

    // Connect to target WebSocket
    const targetWs = new WebSocket(targetUrl);

    targetWs.onopen = () => {
      console.log(`[WebSocket] Connected to target`);
      ws.data.targetWs = targetWs;
    };

    targetWs.onmessage = (event) => {
      // Forward message from target to client
      if (typeof event.data === "string") {
        ws.send(event.data);
      } else if (event.data instanceof ArrayBuffer) {
        ws.send(new Uint8Array(event.data));
      } else if (event.data instanceof Blob) {
        event.data.arrayBuffer().then((buffer) => {
          ws.send(new Uint8Array(buffer));
        });
      }
    };

    targetWs.onerror = (error) => {
      console.error(`[WebSocket] Target error:`, error);
      ws.close(1011, "Target WebSocket error");
    };

    targetWs.onclose = (event) => {
      console.log(`[WebSocket] Target closed: ${event.code} ${event.reason}`);
      ws.close(event.code, event.reason);
    };
  },

  message(ws: ServerWebSocket<WSData>, message: string | Buffer) {
    // Forward message from client to target
    const { targetWs } = ws.data;
    if (targetWs && targetWs.readyState === WebSocket.OPEN) {
      if (typeof message === "string") {
        targetWs.send(message);
      } else {
        targetWs.send(message);
      }
    }
  },

  close(ws: ServerWebSocket<WSData>, code: number, reason: string) {
    console.log(`[WebSocket] Client closed: ${code} ${reason}`);
    const { targetWs } = ws.data;
    if (targetWs && targetWs.readyState === WebSocket.OPEN) {
      targetWs.close(code, reason);
    }
  },

  error(ws: ServerWebSocket<WSData>, error: Error) {
    console.error(`[WebSocket] Client error:`, error);
    const { targetWs } = ws.data;
    if (targetWs && targetWs.readyState === WebSocket.OPEN) {
      targetWs.close(1011, "Client WebSocket error");
    }
  },
};

export type { WSData };
