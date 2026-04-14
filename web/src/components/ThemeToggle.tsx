import { useThemeContext } from "../context/ThemeContext";

export function ThemeToggle() {
  const { theme, toggleTheme } = useThemeContext();
  return (
    <button
      onClick={toggleTheme}
      title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
      className="px-2 py-2 text-slate-400 hover:text-slate-600 dark:text-slate-400 dark:hover:text-slate-200 border border-slate-200 hover:border-slate-400 dark:border-slate-700 dark:hover:border-slate-500 rounded-lg transition-colors text-base leading-none"
    >
      {theme === "dark" ? "☀" : "🌙"}
    </button>
  );
}
