import type { Session } from "../api";

const colors: Record<Session["status"], string> = {
  starting:
    "bg-amber-50 text-amber-700 border-amber-200 dark:bg-yellow-500/20 dark:text-yellow-300 dark:border-yellow-500/40",
  idle: "bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-green-500/20 dark:text-green-300 dark:border-green-500/40",
  busy: "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-500/20 dark:text-blue-300 dark:border-blue-500/40",
  waiting:
    "bg-orange-50 text-orange-700 border-orange-300 dark:bg-orange-500/20 dark:text-orange-300 dark:border-orange-500/50",
  stopped:
    "bg-slate-100 text-slate-500 border-slate-200 dark:bg-slate-500/20 dark:text-slate-400 dark:border-slate-500/40",
  error:
    "bg-red-50 text-red-700 border-red-200 dark:bg-red-500/20 dark:text-red-300 dark:border-red-500/40",
};

const dots: Record<Session["status"], string> = {
  starting: "animate-pulse bg-amber-500 dark:bg-yellow-400",
  idle: "bg-emerald-500 dark:bg-green-400",
  busy: "animate-ping bg-blue-500 dark:bg-blue-400",
  waiting: "animate-pulse bg-orange-500 dark:bg-orange-400",
  stopped: "bg-slate-400 dark:bg-slate-500",
  error: "bg-red-500 dark:bg-red-400",
};

export function StatusBadge({ status }: { status: Session["status"] }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded border text-xs font-mono ${colors[status]}`}
    >
      <span className={`inline-block w-1.5 h-1.5 rounded-full ${dots[status]}`} />
      {status}
    </span>
  );
}
