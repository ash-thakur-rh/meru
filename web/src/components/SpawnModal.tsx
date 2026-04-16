import { useState, useEffect, type FormEvent } from "react";
import { api, type SpawnParams, type NodeInfo } from "../api";
import { FileBrowser } from "./FileBrowser";

const AGENTS = ["claude", "opencode", "goose", "aider"];

interface Props {
  onClose: () => void;
  onSpawned: () => void;
  /** Pre-fill the form for re-spawning a previous session. */
  initial?: { agent?: string; workspace?: string; model?: string; node?: string };
}

export function SpawnModal({ onClose, onSpawned, initial }: Props) {
  const [form, setForm] = useState<SpawnParams>({
    agent: initial?.agent ?? "claude",
    workspace: initial?.workspace ?? "",
    node: initial?.node,
  });
  const [model, setModel] = useState(initial?.model ?? "");
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [showBrowser, setShowBrowser] = useState(false);

  // Git clone state
  const [cloneEnabled, setCloneEnabled] = useState(false);
  const [gitURL, setGitURL] = useState("");
  const [gitDest, setGitDest] = useState("");
  const [gitUsername, setGitUsername] = useState("");
  const [gitPassword, setGitPassword] = useState("");
  const [showCreds, setShowCreds] = useState(false);

  // Worktree state
  const [worktree, setWorktree] = useState(false);
  const [branchName, setBranchName] = useState("");
  const [branchEdited, setBranchEdited] = useState(false);

  // Submission state
  const [cloning, setCloning] = useState(false);
  const [cloneError, setCloneError] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .listNodes()
      .then(setNodes)
      .catch(() => {});
  }, []);

  const set = (k: keyof SpawnParams, v: string | boolean) => setForm((f) => ({ ...f, [k]: v }));

  function slugify(name: string): string {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 50)
      .replace(/-+$/, "");
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setCloneError("");

    let workspace = form.workspace ?? "";

    // Step 1: clone if requested
    if (cloneEnabled) {
      if (!gitURL.trim()) {
        setCloneError("Git URL is required when cloning is enabled.");
        return;
      }
      setCloning(true);
      try {
        const result = await api.gitClone({
          url: gitURL.trim(),
          dest: gitDest.trim() || undefined,
          node: form.node || undefined,
          username: gitUsername || undefined,
          password: gitPassword || undefined,
        });
        workspace = result.path;
      } catch (err) {
        setCloneError((err as Error).message);
        setCloning(false);
        return;
      }
      setCloning(false);
    }

    // Step 2: spawn
    setLoading(true);
    try {
      await api.spawnSession({
        ...form,
        workspace,
        model: model || undefined,
        worktree,
        branch_name: worktree && branchName ? branchName : undefined,
      });
      onSpawned();
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  const inputCls =
    "w-full bg-slate-100 border border-slate-300 dark:bg-slate-800 dark:border-slate-600 rounded px-3 py-2 text-slate-900 dark:text-slate-100 font-mono text-sm focus:outline-none focus:border-purple-500 placeholder:text-slate-400 dark:placeholder:text-slate-600";

  const isBusy = cloning || loading;
  const busyLabel = cloning ? "Cloning…" : loading ? "Starting agent…" : "Spawn";

  return (
    <div
      className="fixed inset-0 bg-black/40 dark:bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <form
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        className="bg-white border border-slate-200 dark:bg-slate-900 dark:border-slate-700 rounded-xl p-6 w-full max-w-md shadow-2xl overflow-y-auto"
        style={{ maxHeight: "90vh" }}
      >
        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 mb-4">
          Spawn new session
        </h2>

        {/* Agent */}
        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Agent</span>
          <select
            value={form.agent}
            onChange={(e) => set("agent", e.target.value)}
            className={inputCls}
          >
            {AGENTS.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>

        {/* Node */}
        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Node</span>
          <select
            value={form.node ?? ""}
            onChange={(e) => set("node", e.target.value)}
            className={inputCls}
          >
            <option value="">local</option>
            {nodes.map((n) => (
              <option key={n.name} value={n.name}>
                {n.name} ({n.addr})
              </option>
            ))}
          </select>
        </label>

        {/* Name */}
        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
            Name <span className="text-slate-400 dark:text-slate-600">(optional)</span>
          </span>
          <input
            type="text"
            value={form.name ?? ""}
            onChange={(e) => {
              set("name", e.target.value);
              if (!branchEdited) {
                setBranchName(slugify(e.target.value));
              }
            }}
            placeholder="auto-generated"
            className={inputCls}
          />
        </label>

        {/* Model */}
        <label className="block mb-4">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
            Model <span className="text-slate-400 dark:text-slate-600">(optional)</span>
          </span>
          <input
            type="text"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="e.g. claude-opus-4-6"
            className={inputCls}
          />
        </label>

        {/* ── Git clone section ── */}
        <div className="border border-slate-200 dark:border-slate-700 rounded-lg mb-4">
          <button
            type="button"
            onClick={() => setCloneEnabled((v) => !v)}
            className="w-full flex items-center justify-between px-3 py-2.5 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-lg transition-colors"
          >
            <span className="flex items-center gap-2">
              <span>⎘</span> Clone from git repository
            </span>
            <span className="text-slate-400 dark:text-slate-500 text-xs">
              {cloneEnabled ? "▲" : "▼"}
            </span>
          </button>

          {cloneEnabled && (
            <div className="px-3 pb-3 space-y-3 border-t border-slate-200 dark:border-slate-700 pt-3">
              {/* Git URL */}
              <label className="block">
                <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
                  Repository URL
                </span>
                <input
                  type="text"
                  value={gitURL}
                  onChange={(e) => setGitURL(e.target.value)}
                  placeholder="https://github.com/org/repo.git"
                  className={inputCls}
                />
              </label>

              {/* Clone destination */}
              <label className="block">
                <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
                  Clone to{" "}
                  <span className="text-slate-400 dark:text-slate-600">
                    (empty = ~/meru-workspaces/&lt;repo&gt;)
                  </span>
                </span>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={gitDest}
                    onChange={(e) => setGitDest(e.target.value)}
                    placeholder="auto-generated"
                    className={inputCls}
                  />
                  <button
                    type="button"
                    onClick={() => setShowBrowser(true)}
                    title="Browse"
                    className="shrink-0 px-2.5 py-2 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100 border border-slate-300 dark:border-slate-600 hover:border-slate-400 dark:hover:border-slate-400 rounded transition-colors text-sm"
                  >
                    📂
                  </button>
                </div>
              </label>

              {/* Credentials toggle */}
              <button
                type="button"
                onClick={() => setShowCreds((v) => !v)}
                className="text-xs text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 flex items-center gap-1"
              >
                <span>{showCreds ? "▲" : "▼"}</span>
                Private repository credentials
              </button>

              {showCreds && (
                <div className="space-y-2 pl-2 border-l-2 border-slate-200 dark:border-slate-700">
                  <p className="text-xs text-slate-400 dark:text-slate-500">
                    For HTTPS repos only. Use a personal access token as the password. SSH repos use
                    your local key agent — no credentials needed here.
                  </p>
                  <label className="block">
                    <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
                      Username
                    </span>
                    <input
                      type="text"
                      value={gitUsername}
                      onChange={(e) => setGitUsername(e.target.value)}
                      placeholder="git username or oauth2"
                      className={inputCls}
                      autoComplete="off"
                    />
                  </label>
                  <label className="block">
                    <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
                      Password / token
                    </span>
                    <input
                      type="password"
                      value={gitPassword}
                      onChange={(e) => setGitPassword(e.target.value)}
                      placeholder="personal access token"
                      className={inputCls}
                      autoComplete="new-password"
                    />
                  </label>
                </div>
              )}

              {cloneError && (
                <p className="text-red-600 dark:text-red-400 text-xs break-all">{cloneError}</p>
              )}
            </div>
          )}
        </div>

        {/* Workspace (manual path, shown when not cloning or as override) */}
        {!cloneEnabled && (
          <label className="block mb-4">
            <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Workspace</span>
            <div className="flex gap-2">
              <input
                type="text"
                value={form.workspace ?? ""}
                onChange={(e) => set("workspace", e.target.value)}
                placeholder="/path/to/project"
                className={inputCls}
              />
              <button
                type="button"
                onClick={() => setShowBrowser(true)}
                title="Browse filesystem"
                className="shrink-0 px-2.5 py-2 text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100 border border-slate-300 dark:border-slate-600 hover:border-slate-400 dark:hover:border-slate-400 rounded transition-colors text-sm"
              >
                📂
              </button>
            </div>
          </label>
        )}

        {/* ── Git worktree section ── */}
        <div className="border border-slate-200 dark:border-slate-700 rounded-lg mb-5">
          <label className="flex items-center gap-3 px-3 py-2.5 cursor-pointer">
            <input
              type="checkbox"
              checked={worktree}
              onChange={(e) => setWorktree(e.target.checked)}
              className="accent-purple-500"
            />
            <div>
              <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                Create isolated git worktree
              </span>
              <p className="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
                Spawns the agent in a fresh branch so its changes stay isolated. Requires the
                workspace to be a git repository.
              </p>
            </div>
          </label>
        </div>

        {worktree && (
          <label className="block mb-4">
            <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
              Branch name{" "}
              <span className="text-slate-400 dark:text-slate-600">(optional)</span>
            </span>
            <input
              type="text"
              value={branchName}
              onChange={(e) => {
                setBranchName(e.target.value);
                setBranchEdited(e.target.value !== "");
              }}
              placeholder={form.name ? slugify(form.name) : "auto-derived from session name"}
              className={inputCls}
            />
            <p className="text-xs text-slate-400 dark:text-slate-500 mt-1">
              Branch will be created as{" "}
              <code className="font-mono">meru/{branchName || slugify(form.name ?? "") || "…"}</code>
            </p>
          </label>
        )}

        {loading && (
          <p className="text-xs text-slate-400 dark:text-slate-500 mb-3">
            Starting the agent process — this may take a few seconds…
          </p>
        )}

        {error && <p className="text-red-600 dark:text-red-400 text-sm mb-3">{error}</p>}

        <div className="flex gap-2 justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={isBusy}
            className="px-4 py-2 text-sm text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 transition-colors disabled:opacity-40"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isBusy || !form.agent || (cloneEnabled && !gitURL.trim())}
            className="px-4 py-2 text-sm bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white rounded-lg transition-colors min-w-20"
          >
            {busyLabel}
          </button>
        </div>
      </form>

      {showBrowser && (
        <FileBrowser
          node={form.node || "local"}
          initialPath={cloneEnabled ? gitDest : form.workspace || ""}
          onSelect={(path) => {
            if (cloneEnabled) {
              setGitDest(path);
            } else {
              set("workspace", path);
            }
            setShowBrowser(false);
          }}
          onClose={() => setShowBrowser(false)}
        />
      )}
    </div>
  );
}
