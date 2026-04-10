import { createContext, PropsWithChildren, useContext, useEffect, useMemo, useState } from "react";

type ThemeMode = "light" | "dark" | "system";
type Accent = "slate" | "blue" | "green" | "violet";

type ThemeContextValue = {
  mode: ThemeMode;
  accent: Accent;
  setMode: (mode: ThemeMode) => void;
  setAccent: (accent: Accent) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

const storageKeys = {
  mode: "app.theme.mode",
  accent: "app.theme.accent"
};

function resolveMode(mode: ThemeMode) {
  if (mode !== "system") {
    return mode;
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: PropsWithChildren) {
  const [mode, setMode] = useState<ThemeMode>(() => (localStorage.getItem(storageKeys.mode) as ThemeMode) || "system");
  const [accent, setAccent] = useState<Accent>(() => (localStorage.getItem(storageKeys.accent) as Accent) || "slate");

  useEffect(() => {
    localStorage.setItem(storageKeys.mode, mode);
    document.documentElement.dataset.theme = resolveMode(mode);
  }, [mode]);

  useEffect(() => {
    localStorage.setItem(storageKeys.accent, accent);
    document.documentElement.dataset.accent = accent;
  }, [accent]);

  const value = useMemo(() => ({ mode, accent, setMode, setAccent }), [accent, mode]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return value;
}

