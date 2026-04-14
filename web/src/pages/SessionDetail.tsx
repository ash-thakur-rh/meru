import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { api, type Session } from "../api";
import { AgentTerminal } from "../components/AgentTerminal";
import { StatusBadge } from "../components/StatusBadge";
import { ThemeToggle } from "../components/ThemeToggle";
import { SpawnModal } from "../components/SpawnModal";
import { useThemeContext } from "../context/ThemeContext";

export function SessionDetail() {
  const { id } = useParams<{ id: string }>();
  const nav = useNavigate();
  const [session, setSession] = useState<Session | null>(null);
  const [prompt, setPrompt] = useState("");
  const [showRespawn, setShowRespawn] = useState(false);
  // writeInput is provided by AgentTerminal once the terminal WS opens.
  const writeInputRef = useRef<((data: string) => void) | null>(null);
  const { theme } = useThemeContext();

  const termBg = theme === "dark" ? "#0f1117" : "#f8fafc";

  // Load session info (works for both live and stopped sessions).
  useEffect(() => {
    if (!id) return;
    api
      .getSession(id)
      .then(setSession)
      .catch(() => nav("/"));
  }, [id, nav]);

  // Periodically refresh status while the page is open.
  useEffect(() => {
    if (!id) return;
    const t = setInterval(() => {
      api
        .getSession(id)
        .then(setSession)
        .catch(() => {});
    }, 5000);
    return () => clearInterval(t);
  }, [id]);

  function sendPrompt() {
    if (!prompt.trim() || !writeInputRef.current) return;
    writeInputRef.current(prompt + "\r");
    setPrompt("");
  }

  function onKey(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendPrompt();
    }
  }

  async function stop() {
    if (!id) return;
    await api.stopSession(id);
    setSession((s) => (s ? { ...s, status: "stopped" } : s));
  }

  const isStopped = session?.status === "stopped";

  return (
    <div className="flex flex-col h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100">
      {/* Header */}
      <header className="flex-none border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 px-4 py-3 flex items-center gap-3">
        <button
          onClick={() => nav("/")}
          className="text-slate-400 hover:text-slate-900 dark:text-slate-500 dark:hover:text-slate-200 transition-colors text-sm px-2 py-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          ← Back
        </button>

        <div className="flex-1 min-w-0 flex items-center gap-2">
          <span className="font-semibold truncate">{session?.name ?? "…"}</span>
          {session && <StatusBadge status={session.status} />}
          <span className="text-xs text-slate-400 dark:text-slate-500 font-mono hidden sm:block">
            {session?.agent}
          </span>
        </div>

        <div className="text-xs text-slate-400 dark:text-slate-600 font-mono hidden md:block truncate max-w-48">
          {session?.workspace}
        </div>

        {session && !isStopped && (
          <button
            onClick={stop}
            className="text-xs text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 px-3 py-1.5 rounded border border-red-300 hover:border-red-400 dark:border-red-800 dark:hover:border-red-600 transition-all"
          >
            Stop
          </button>
        )}

        {session && isStopped && (
          <button
            onClick={() => setShowRespawn(true)}
            className="text-xs text-purple-600 hover:text-purple-700 dark:text-purple-400 dark:hover:text-purple-300 px-3 py-1.5 rounded border border-purple-300 hover:border-purple-400 dark:border-purple-800 dark:hover:border-purple-600 transition-all"
          >
            Re-spawn
          </button>
        )}

        <ThemeToggle />
      </header>

      {/* Stopped banner */}
      {isStopped && (
        <div className="flex-none px-4 py-2 bg-slate-100 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 text-xs text-slate-500 dark:text-slate-400 flex items-center gap-2">
          <span className="w-1.5 h-1.5 rounded-full bg-slate-400 dark:bg-slate-500 inline-block" />
          Session stopped — terminal shows the log history. Hit Re-spawn to start a new session in
          the same workspace.
        </div>
      )}

      {/* Terminal — fills remaining space */}
      <div className="flex-1 min-h-0" style={{ background: termBg }}>
        {id && (
          <AgentTerminal
            sessionId={id}
            onReady={(write) => {
              writeInputRef.current = write;
            }}
          />
        )}
      </div>

      {/* Send bar — only for live sessions */}
      {!isStopped && (
        <div className="flex-none border-t border-slate-200 dark:border-slate-800 px-4 py-3 bg-white dark:bg-slate-900">
          <div className="flex gap-2 items-end max-w-5xl mx-auto">
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              onKeyDown={onKey}
              rows={1}
              placeholder="Paste a prompt here and press Enter — or type directly in the terminal above"
              className="flex-1 bg-slate-100 border border-slate-300 focus:border-purple-500 dark:bg-slate-800 dark:border-slate-700 dark:focus:border-purple-500 rounded-lg px-3 py-2 text-sm text-slate-900 dark:text-slate-100 font-mono resize-none placeholder:text-slate-400 dark:placeholder:text-slate-600 focus:outline-none transition-colors"
              style={{ minHeight: 40, maxHeight: 140 }}
              onInput={(e) => {
                const el = e.currentTarget;
                el.style.height = "auto";
                el.style.height = el.scrollHeight + "px";
              }}
            />
            <button
              onClick={sendPrompt}
              disabled={!prompt.trim()}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-500 disabled:opacity-40 text-white text-sm rounded-lg transition-colors h-10"
            >
              Send
            </button>
          </div>
          <p className="text-xs text-slate-400 dark:text-slate-600 mt-1 max-w-5xl mx-auto">
            Session · <span className="font-mono">{id}</span>
          </p>
        </div>
      )}

      {showRespawn && session && (
        <SpawnModal
          onClose={() => setShowRespawn(false)}
          onSpawned={() => {
            setShowRespawn(false);
            nav("/");
          }}
          initial={{
            agent: session.agent,
            workspace: session.workspace,
            model: session.model,
            node: session.node_name !== "local" ? session.node_name : undefined,
          }}
        />
      )}
    </div>
  );
}
