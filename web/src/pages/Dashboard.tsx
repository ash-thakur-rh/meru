import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useSessions } from "../hooks/useSessions";
import { SessionCard } from "../components/SessionCard";
import { SpawnModal } from "../components/SpawnModal";
import { ThemeToggle } from "../components/ThemeToggle";
import { api, type Session } from "../api";

export function Dashboard() {
  const { sessions, error, refresh } = useSessions();
  const [showSpawn, setShowSpawn] = useState(false);
  const [respawnSeed, setRespawnSeed] = useState<Session | null>(null);
  const nav = useNavigate();

  async function stop(id: string) {
    await api.stopSession(id);
    refresh();
  }

  async function deleteSession(id: string) {
    await api.deleteSession(id);
    refresh();
  }

  function respawn(session: Session) {
    setRespawnSeed(session);
    setShowSpawn(true);
  }

  const active = sessions
    .filter((s) => s.status !== "stopped")
    .sort((a, b) => (b.status === "waiting" ? 1 : 0) - (a.status === "waiting" ? 1 : 0));
  const stopped = sessions.filter((s) => s.status === "stopped");

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100">
      {/* Header */}
      <header className="border-b border-slate-200 dark:border-slate-800 px-6 py-4 flex items-center justify-between bg-white dark:bg-slate-950">
        <div className="flex items-center gap-3">
          <span className="text-2xl">🎼</span>
          <div>
            <h1 className="text-lg font-semibold leading-none">Conductor</h1>
            <p className="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
              Local agent orchestrator
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <button
            onClick={() => nav("/nodes")}
            className="px-3 py-2 text-sm text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 border border-slate-200 hover:border-slate-400 dark:border-slate-700 dark:hover:border-slate-500 rounded-lg transition-colors"
          >
            ⬡ Nodes
          </button>
          <button
            onClick={() => {
              setRespawnSeed(null);
              setShowSpawn(true);
            }}
            className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white text-sm rounded-lg transition-colors"
          >
            <span className="text-base leading-none">＋</span>
            Spawn agent
          </button>
        </div>
      </header>

      <main className="px-6 py-6 max-w-6xl mx-auto">
        {error && (
          <div className="mb-4 px-4 py-3 bg-red-50 border border-red-200 dark:bg-red-900/30 dark:border-red-800 rounded-lg text-red-700 dark:text-red-300 text-sm">
            Cannot reach daemon — is <code className="font-mono">meru serve</code> running?
          </div>
        )}

        {/* Active sessions */}
        <section className="mb-8">
          <h2 className="text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-widest mb-3">
            Active · {active.length}
          </h2>
          {active.length === 0 ? (
            <div className="border border-dashed border-slate-300 dark:border-slate-700 rounded-lg p-8 text-center text-slate-400 dark:text-slate-600 text-sm">
              No active sessions.{" "}
              <button
                onClick={() => {
                  setRespawnSeed(null);
                  setShowSpawn(true);
                }}
                className="text-purple-600 hover:text-purple-500 dark:text-purple-400 dark:hover:text-purple-300 underline"
              >
                Spawn one
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {active.map((s) => (
                <SessionCard key={s.id} session={s} onStop={stop} />
              ))}
            </div>
          )}
        </section>

        {/* Recent / stopped sessions */}
        {stopped.length > 0 && (
          <section>
            <h2 className="text-xs font-semibold text-slate-400 dark:text-slate-600 uppercase tracking-widest mb-3">
              Recent · {stopped.length}
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {stopped.map((s) => (
                <SessionCard
                  key={s.id}
                  session={s}
                  onStop={stop}
                  onDelete={deleteSession}
                  onRespawn={respawn}
                />
              ))}
            </div>
            <p className="mt-3 text-xs text-slate-400 dark:text-slate-600">
              Hover a card to re-spawn with the same config, or delete it permanently.
            </p>
          </section>
        )}
      </main>

      {showSpawn && (
        <SpawnModal
          onClose={() => {
            setShowSpawn(false);
            setRespawnSeed(null);
          }}
          onSpawned={() => {
            refresh();
            setRespawnSeed(null);
          }}
          initial={
            respawnSeed
              ? {
                  agent: respawnSeed.agent,
                  workspace: respawnSeed.workspace,
                  model: respawnSeed.model,
                  node: respawnSeed.node_name !== "local" ? respawnSeed.node_name : undefined,
                }
              : undefined
          }
        />
      )}
    </div>
  );
}
