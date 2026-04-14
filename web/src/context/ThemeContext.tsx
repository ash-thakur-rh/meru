import { createContext, useContext } from "react";
import type { Theme } from "../hooks/useTheme";

interface ThemeCtx {
  theme: Theme;
  toggleTheme: () => void;
}

export const ThemeContext = createContext<ThemeCtx>({
  theme: "dark",
  toggleTheme: () => {},
});

export function useThemeContext() {
  return useContext(ThemeContext);
}
