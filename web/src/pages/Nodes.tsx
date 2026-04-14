import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api, type NodeInfo } from "../api";
import { ThemeToggle } from "../components/ThemeToggle";

export function Nodes() {
  const nav = useNavigate();
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [showAdd, setShowAdd] = useState(false);
  const [pinging, setPinging] = useState<string | null>(null);
  const [pingResults, setPingResults] = useState<Record<string, NodeInfo>>({});

  const load = () =>
    api
      .listNodes()
      .then(setNodes)
      .catch(() => {});
  useEffect(() => {
    load();
  }, []);

  async function pingNode(name: string) {
    setPinging(name);
    try {
      const info = await api.pingNode(name);
      setPingResults((r) => ({ ...r, [name]: info }));
    } catch {
      setPingResults((r) => ({
        ...r,
        [name]: { name, addr: "", tls: false, version: "unreachable" },
      }));
    } finally {
      setPinging(null);
    }
  }

  async function remove(name: string) {
    await api.removeNode(name);
    load();
  }

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100">
      <header className="border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 px-6 py-4 flex items-center gap-3">
        <button
          onClick={() => nav("/")}
          className="text-slate-400 hover:text-slate-900 dark:text-slate-500 dark:hover:text-slate-200 text-sm px-2 py-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          ← Back
        </button>
        <div className="flex-1">
          <h1 className="text-lg font-semibold">Remote Nodes</h1>
          <p className="text-xs text-slate-400 dark:text-slate-500">meru-node targets</p>
        </div>
        <ThemeToggle />
        <button
          onClick={() => setShowAdd(true)}
          className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm rounded-lg"
        >
          + Add node
        </button>
      </header>

      <main className="px-6 py-6 max-w-4xl mx-auto">
        {/* Local node */}
        <section className="mb-6">
          <h2 className="text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-widest mb-3">
            Built-in
          </h2>
          <div className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/40 p-4 flex items-center justify-between">
            <div>
              <div className="font-semibold">local</div>
              <div className="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
                This machine · in-process
              </div>
            </div>
            <span className="text-xs bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-green-900/30 dark:text-green-400 dark:border-green-700/40 px-2 py-0.5 rounded font-mono">
              online
            </span>
          </div>
        </section>

        {/* Remote nodes */}
        <section>
          <h2 className="text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-widest mb-3">
            Remote · {nodes.length}
          </h2>
          {nodes.length === 0 ? (
            <div className="border border-dashed border-slate-300 dark:border-slate-700 rounded-lg p-8 text-center text-slate-400 dark:text-slate-600 text-sm">
              No remote nodes.{" "}
              <button
                onClick={() => setShowAdd(true)}
                className="text-violet-600 hover:text-violet-500 dark:text-violet-400 dark:hover:text-violet-300 underline"
              >
                Add one
              </button>
            </div>
          ) : (
            <div className="space-y-3">
              {nodes.map((n) => {
                const ping = pingResults[n.name];
                return (
                  <div
                    key={n.name}
                    className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/40 p-4"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <div className="font-semibold">{n.name}</div>
                        <div className="text-xs text-slate-400 dark:text-slate-500 font-mono mt-0.5">
                          {n.tls ? "grpcs" : "grpc"}://{n.addr}
                        </div>
                        {ping && (
                          <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                            {ping.version === "unreachable" ? (
                              <span className="text-red-600 dark:text-red-400">unreachable</span>
                            ) : (
                              <>
                                <span className="text-emerald-600 dark:text-green-400">
                                  ✓ online
                                </span>
                                {" · "}
                                {ping.version}
                                {ping.agents && <> · agents: {ping.agents.join(", ")}</>}
                              </>
                            )}
                          </div>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => pingNode(n.name)}
                          disabled={pinging === n.name}
                          className="text-xs text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 px-3 py-1.5 rounded border border-slate-300 hover:border-slate-400 dark:border-slate-600 dark:hover:border-slate-400 transition-all disabled:opacity-50"
                        >
                          {pinging === n.name ? "…" : "ping"}
                        </button>
                        <button
                          onClick={() => remove(n.name)}
                          className="text-xs text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 px-3 py-1.5 rounded border border-red-300 hover:border-red-400 dark:border-red-800 dark:hover:border-red-600 transition-all"
                        >
                          remove
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </section>
      </main>

      {showAdd && <AddNodeModal onClose={() => setShowAdd(false)} onAdded={load} />}
    </div>
  );
}

function AddNodeModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [name, setName] = useState("");
  const [addr, setAddr] = useState("");
  const [token, setToken] = useState("");
  const [tls, setTls] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await api.addNode({ name, addr, token, tls });
      onAdded();
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  const inputCls =
    "w-full bg-slate-100 border border-slate-300 dark:bg-slate-800 dark:border-slate-600 rounded px-3 py-2 text-slate-900 dark:text-slate-100 font-mono text-sm focus:outline-none focus:border-violet-500 placeholder:text-slate-400 dark:placeholder:text-slate-600";

  return (
    <div
      className="fixed inset-0 bg-black/40 dark:bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <form
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        className="bg-white border border-slate-200 dark:bg-slate-900 dark:border-slate-700 rounded-xl p-6 w-full max-w-md shadow-2xl"
      >
        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 mb-4">
          Add remote node
        </h2>

        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Name</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="gpu-box"
            className={inputCls}
          />
        </label>

        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">
            Address <span className="text-slate-400 dark:text-slate-600">host:port</span>
          </span>
          <input
            value={addr}
            onChange={(e) => setAddr(e.target.value)}
            placeholder="10.0.0.5:9090"
            className={inputCls}
          />
        </label>

        <label className="block mb-3">
          <span className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Token</span>
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="shared secret"
            className={inputCls}
          />
        </label>

        <label className="flex items-center gap-2 mb-5 cursor-pointer">
          <input
            type="checkbox"
            checked={tls}
            onChange={(e) => setTls(e.target.checked)}
            className="accent-violet-500"
          />
          <span className="text-sm text-slate-700 dark:text-slate-300">Use TLS</span>
        </label>

        {error && <p className="text-red-600 dark:text-red-400 text-sm mb-3">{error}</p>}

        <div className="flex gap-2 justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading || !name || !addr || !token}
            className="px-4 py-2 text-sm bg-violet-600 hover:bg-violet-500 disabled:opacity-50 text-white rounded-lg"
          >
            {loading ? "Adding…" : "Add node"}
          </button>
        </div>
      </form>
    </div>
  );
}
