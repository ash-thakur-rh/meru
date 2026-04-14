import { useNavigate } from "react-router-dom";
import type { Session } from "../api";
import { StatusBadge } from "./StatusBadge";
import { NodeBadge } from "./NodeBadge";

const agentIcons: Record<string, string> = {
  claude: "🟣",
  opencode: "🟡",
  goose: "🪿",
  aider: "🤖",
};

interface Props {
  session: Session;
  onStop: (id: string) => void;
  onDelete?: (id: string) => void;
  onRespawn?: (session: Session) => void;
}

export function SessionCard({ session, onStop, onDelete, onRespawn }: Props) {
  const nav = useNavigate();
  const icon = agentIcons[session.agent] ?? "⚡";
  const isStopped = session.status === "stopped";
  const isWaiting = session.status === "waiting";

  return (
    <div
      onClick={() => nav(`/sessions/${session.id}`)}
      className={`cursor-pointer rounded-lg border bg-white dark:bg-slate-800/60 transition-all group ${
        isStopped
          ? "border-slate-200 dark:border-slate-700/50 opacity-70 hover:opacity-90"
          : isWaiting
            ? "border-orange-400 dark:border-orange-500/70 ring-2 ring-orange-300/50 dark:ring-orange-500/30 hover:border-orange-500 dark:hover:border-orange-400"
            : "border-slate-200 hover:border-slate-400 hover:bg-slate-50 dark:border-slate-700 dark:hover:border-slate-500 dark:hover:bg-slate-800"
      }`}
    >
      <div className="p-4">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-xl">{icon}</span>
            <div className="min-w-0">
              <div className="font-semibold text-slate-900 dark:text-slate-100 truncate">
                {session.name}
              </div>
              <div className="text-xs text-slate-500 dark:text-slate-400 font-mono truncate">
                {session.agent}
              </div>
            </div>
          </div>
          <StatusBadge status={session.status} />
        </div>

        <div className="mt-2 flex items-center gap-2">
          <NodeBadge node={session.node_name} />
        </div>
        <div className="mt-2 text-xs text-slate-400 dark:text-slate-500 font-mono truncate">
          {session.workspace}
        </div>

        <div className="mt-3 flex items-center justify-between gap-2">
          <div className="text-xs text-slate-400 dark:text-slate-600 font-mono">
            {session.id.slice(0, 8)}
          </div>

          {/* Active session: stop button */}
          {!isStopped && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onStop(session.id);
              }}
              className="opacity-0 group-hover:opacity-100 text-xs text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 px-2 py-0.5 rounded border border-red-300 hover:border-red-400 dark:border-red-800 dark:hover:border-red-600 transition-all"
            >
              stop
            </button>
          )}

          {/* Stopped session: re-spawn + delete */}
          {isStopped && (
            <div className="flex items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-all">
              {onRespawn && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onRespawn(session);
                  }}
                  className="text-xs text-purple-600 hover:text-purple-700 dark:text-purple-400 dark:hover:text-purple-300 px-2 py-0.5 rounded border border-purple-300 hover:border-purple-400 dark:border-purple-800 dark:hover:border-purple-600 transition-colors"
                >
                  re-spawn
                </button>
              )}
              {onDelete && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onDelete(session.id);
                  }}
                  className="text-xs text-slate-400 hover:text-red-600 dark:text-slate-500 dark:hover:text-red-400 px-2 py-0.5 rounded border border-slate-200 hover:border-red-300 dark:border-slate-700 dark:hover:border-red-800 transition-colors"
                >
                  delete
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
