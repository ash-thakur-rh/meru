import { useState, useEffect, useCallback } from "react";
import { api, type DirListing, type DirEntry } from "../api";

interface Props {
  /** Which node's filesystem to browse ("local" or a registered remote node name) */
  node: string;
  /** Initial path; defaults to the node's home directory */
  initialPath?: string;
  /** Called when the user confirms a directory selection */
  onSelect: (path: string) => void;
  onClose: () => void;
}

export function FileBrowser({ node, initialPath, onSelect, onClose }: Props) {
  const [listing, setListing] = useState<DirListing | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const navigate = useCallback(
    (path: string) => {
      setLoading(true);
      setError("");
      api
        .listDir(path, node === "local" ? undefined : node)
        .then(setListing)
        .catch((e: Error) => setError(e.message))
        .finally(() => setLoading(false));
    },
    [node]
  );

  // Load initial path on mount
  useEffect(() => {
    navigate(initialPath ?? "");
  }, [navigate, initialPath]);

  const pathParts = listing ? splitPath(listing.path) : [];

  return (
    <div
      className="fixed inset-0 bg-black/40 dark:bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-white border border-slate-200 dark:bg-slate-900 dark:border-slate-700 rounded-xl shadow-2xl w-full max-w-lg flex flex-col"
        style={{ maxHeight: "80vh" }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-700 flex-none">
          <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">
            Browse filesystem
            {node !== "local" && (
              <span className="ml-2 text-xs font-normal text-violet-600 dark:text-violet-400">
                ⬡ {node}
              </span>
            )}
          </span>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 text-lg leading-none"
          >
            ✕
          </button>
        </div>

        {/* Breadcrumb */}
        {listing && (
          <div className="flex items-center gap-1 px-4 py-2 border-b border-slate-100 dark:border-slate-800 flex-none overflow-x-auto scrollbar-none">
            {pathParts.map((part, i) => (
              <span key={i} className="flex items-center gap-1 shrink-0">
                {i > 0 && <span className="text-slate-300 dark:text-slate-600 text-xs">/</span>}
                <button
                  onClick={() => navigate(part.fullPath)}
                  className="text-xs font-mono text-slate-600 hover:text-purple-600 dark:text-slate-400 dark:hover:text-purple-400 truncate max-w-32"
                  title={part.fullPath}
                >
                  {part.label}
                </button>
              </span>
            ))}
          </div>
        )}

        {/* Directory listing */}
        <div className="flex-1 overflow-y-auto min-h-0">
          {loading && (
            <div className="flex items-center justify-center py-12 text-slate-400 dark:text-slate-500 text-sm">
              Loading…
            </div>
          )}
          {error && <div className="px-4 py-3 text-red-600 dark:text-red-400 text-sm">{error}</div>}
          {!loading && listing && (
            <ul className="py-1">
              {/* Parent directory row */}
              {listing.parent && (
                <li>
                  <button
                    onClick={() => navigate(listing.parent)}
                    className="w-full flex items-center gap-2.5 px-4 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors text-sm"
                  >
                    <span className="text-base">↑</span>
                    <span className="text-slate-500 dark:text-slate-400 font-mono">..</span>
                  </button>
                </li>
              )}
              {listing.entries.length === 0 && (
                <li className="px-4 py-6 text-center text-slate-400 dark:text-slate-500 text-sm">
                  Empty directory
                </li>
              )}
              {listing.entries.map((entry) => (
                <EntryRow key={entry.path} entry={entry} onNavigate={navigate} />
              ))}
            </ul>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-slate-200 dark:border-slate-700 flex-none bg-slate-50 dark:bg-slate-900 rounded-b-xl">
          <span className="text-xs font-mono text-slate-400 dark:text-slate-500 truncate max-w-xs">
            {listing?.path ?? ""}
          </span>
          <div className="flex gap-2 shrink-0 ml-3">
            <button
              onClick={onClose}
              className="px-3 py-1.5 text-sm text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            >
              Cancel
            </button>
            <button
              onClick={() => listing && onSelect(listing.path)}
              disabled={!listing}
              className="px-4 py-1.5 text-sm bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white rounded-lg transition-colors"
            >
              Select
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function EntryRow({ entry, onNavigate }: { entry: DirEntry; onNavigate: (path: string) => void }) {
  return (
    <li>
      <button
        onClick={() => entry.is_dir && onNavigate(entry.path)}
        className={`w-full flex items-center gap-2.5 px-4 py-2 text-left transition-colors text-sm ${
          entry.is_dir
            ? "hover:bg-slate-50 dark:hover:bg-slate-800 cursor-pointer"
            : "cursor-default opacity-50"
        }`}
        title={entry.path}
      >
        <span className="text-base shrink-0">{entry.is_dir ? "📁" : "📄"}</span>
        <span
          className={`font-mono truncate ${
            entry.is_dir
              ? "text-slate-800 dark:text-slate-200"
              : "text-slate-500 dark:text-slate-500"
          }`}
        >
          {entry.name}
        </span>
      </button>
    </li>
  );
}

/** Splits an absolute path into breadcrumb parts with labels and full paths. */
function splitPath(abs: string): { label: string; fullPath: string }[] {
  // Normalize separators (Windows paths use \)
  const normalized = abs.replace(/\\/g, "/");
  const segments = normalized.split("/").filter(Boolean);

  if (segments.length === 0) {
    return [{ label: "/", fullPath: "/" }];
  }

  const parts: { label: string; fullPath: string }[] = [{ label: "/", fullPath: "/" }];
  let current = "";
  for (const seg of segments) {
    current += "/" + seg;
    parts.push({ label: seg, fullPath: current });
  }
  return parts;
}
