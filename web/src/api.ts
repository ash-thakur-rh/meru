export interface Session {
  id: string;
  name: string;
  agent: string;
  status: "starting" | "idle" | "busy" | "waiting" | "stopped" | "error";
  workspace: string;
  node_name: string;
  model?: string; // present on stopped sessions returned from store
}

export interface AgentEvent {
  type: "text" | "tool_use" | "tool_result" | "done" | "error";
  text?: string;
  tool_name?: string;
  tool_input?: string;
  error?: string;
  timestamp: string;
}

export interface LogEvent {
  id: number;
  session_id: string;
  type: string;
  text: string;
  tool_name: string;
  tool_input: string;
  error: string;
  timestamp: string;
}

export interface NodeInfo {
  name: string;
  addr: string;
  tls: boolean;
  agents?: string[];
  version?: string;
  last_seen?: string;
}

export interface DirEntry {
  name: string;
  path: string;
  is_dir: boolean;
}

export interface DirListing {
  path: string;
  parent: string;
  entries: DirEntry[];
}

export interface SpawnParams {
  agent: string;
  name?: string;
  workspace?: string;
  model?: string;
  worktree?: boolean;
  node?: string;
  branch_name?: string;
}

const BASE = ""; // same-origin; vite proxy handles /sessions in dev

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error ?? res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  // Nodes
  listNodes: () => request<NodeInfo[]>("GET", "/nodes"),
  addNode: (p: { name: string; addr: string; token: string; tls: boolean }) =>
    request<NodeInfo>("POST", "/nodes", p),
  removeNode: (name: string) => request<void>("DELETE", `/nodes/${name}`),
  pingNode: (name: string) => request<NodeInfo>("POST", `/nodes/${name}/ping`),

  listDir: (path: string, node?: string) => {
    const params = new URLSearchParams({ path });
    if (node) params.set("node", node);
    return request<DirListing>("GET", `/fs?${params}`);
  },

  gitClone: (p: {
    url: string;
    dest?: string;
    node?: string;
    username?: string;
    password?: string;
  }) => request<{ jobId: string }>("POST", "/git/clone/", p),

  cancelClone: (jobId: string) => request<void>("DELETE", `/git/clone/${jobId}`),

  cloneStream: (jobId: string): EventSource => new EventSource(`/git/clone/${jobId}/stream`),

  listSessions: () => request<Session[]>("GET", "/sessions"),
  getSession: (id: string) => request<Session>("GET", `/sessions/${id}`),
  spawnSession: (p: SpawnParams) => request<Session>("POST", "/sessions", p),
  /** Stop a live session (marks it as stopped, keeps in store for history). */
  stopSession: (id: string) => request<void>("DELETE", `/sessions/${id}`),
  /** Purge a stopped session permanently from the store. */
  deleteSession: (id: string) => request<void>("DELETE", `/sessions/${id}`),
  getLogs: (id: string) => request<LogEvent[]>("GET", `/sessions/${id}/logs`),

  /** Returns an async generator that yields AgentEvents from the nd-JSON stream */
  async *send(id: string, prompt: string): AsyncGenerator<AgentEvent> {
    const res = await fetch(`/sessions/${id}/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt }),
    });
    if (!res.ok || !res.body) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      const msg =
        res.status === 409
          ? "Agent is busy — wait for the current response to finish"
          : (err.error ?? `HTTP ${res.status}`);
      throw new Error(msg);
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() ?? "";
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        try {
          yield JSON.parse(trimmed) as AgentEvent;
        } catch {
          // skip malformed lines
        }
      }
    }
  },

  /** Open a WebSocket to stream live events for a session */
  streamSocket(id: string): WebSocket {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const host = location.hostname === "localhost" ? "localhost:8080" : location.host;
    return new WebSocket(`${proto}://${host}/sessions/${id}/stream`);
  },

  /**
   * Open a bidirectional raw-terminal WebSocket for a session.
   * Binary frames carry raw PTY bytes (both directions).
   * Text frames carry JSON resize events: {"type":"resize","cols":N,"rows":N}
   */
  terminalSocket(id: string): WebSocket {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const host = location.hostname === "localhost" ? "localhost:8080" : location.host;
    const ws = new WebSocket(`${proto}://${host}/sessions/${id}/terminal`);
    ws.binaryType = "arraybuffer";
    return ws;
  },
};
