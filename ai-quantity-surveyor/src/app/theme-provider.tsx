"use client";

import { createContext, useContext, useSyncExternalStore, type ReactNode } from "react";

export type Theme = "light" | "dark";

type ThemeState = { theme: Theme; toggleTheme: () => void };
const ThemeContext = createContext<ThemeState>({ theme: "light", toggleTheme: () => {} });
const themeKey = "quantity-surveyor-theme";
const subscribers = new Set<() => void>();
let currentTheme: Theme = "light";
let initialized = false;

function applyTheme(theme: Theme) {
  currentTheme = theme;
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}

function subscribe(callback: () => void) {
  subscribers.add(callback);
  if (!initialized) {
    const saved = localStorage.getItem(themeKey);
    const theme: Theme = saved === "dark" || saved === "light"
      ? saved
      : window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    applyTheme(theme);
    initialized = true;
  }
  window.addEventListener("storage", onStorage);
  callback();
  return () => {
    subscribers.delete(callback);
    window.removeEventListener("storage", onStorage);
  };
}

function onStorage(event: StorageEvent) {
  if (event.key !== themeKey) return;
  applyTheme(event.newValue === "dark" ? "dark" : "light");
  subscribers.forEach((callback) => callback());
}

function getSnapshot(): Theme { return currentTheme; }
function getServerSnapshot(): Theme { return "light"; }

export function useTheme() {
  return useContext(ThemeContext);
}

export default function ThemeProvider({ children }: { children: ReactNode }) {
  const theme = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);

  const toggleTheme = () => {
    const selected: Theme = theme === "dark" ? "light" : "dark";
    applyTheme(selected);
    initialized = true;
    localStorage.setItem(themeKey, selected);
    subscribers.forEach((callback) => callback());
  };

  return <ThemeContext.Provider value={{ theme, toggleTheme }}>{children}</ThemeContext.Provider>;
}
