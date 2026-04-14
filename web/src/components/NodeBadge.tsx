interface Props {
  node: string;
}

export function NodeBadge({ node }: Props) {
  const isLocal = node === "local" || !node;
  return (
    <span
      className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-mono border ${
        isLocal
          ? "bg-slate-100 text-slate-500 border-slate-200 dark:bg-slate-700/40 dark:text-slate-500 dark:border-slate-700"
          : "bg-violet-50 text-violet-700 border-violet-200 dark:bg-violet-900/30 dark:text-violet-300 dark:border-violet-700/50"
      }`}
    >
      {isLocal ? "⬡ local" : `⬡ ${node}`}
    </span>
  );
}
