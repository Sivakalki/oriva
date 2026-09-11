import { createContext, useCallback, useEffect, useMemo, useState, type ReactNode } from "react"

export type Theme = "light" | "dark" | "system"

interface ThemeValue {
  theme: Theme
  /** What's actually applied right now -- resolves "system" to the OS preference. */
  resolvedTheme: "light" | "dark"
  setTheme: (t: Theme) => void
}

export const ThemeContext = createContext<ThemeValue | null>(null)

const STORAGE_KEY = "oriva.theme"
const MEDIA_QUERY = "(prefers-color-scheme: dark)"

function readStoredTheme(): Theme {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw === "light" || raw === "dark" || raw === "system" ? raw : "system"
  } catch {
    return "system"
  }
}

function systemPrefersDark(): boolean {
  return typeof window !== "undefined" && window.matchMedia(MEDIA_QUERY).matches
}

function resolve(theme: Theme): "light" | "dark" {
  return theme === "system" ? (systemPrefersDark() ? "dark" : "light") : theme
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readStoredTheme)
  const resolvedTheme = resolve(theme)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", resolvedTheme === "dark")
  }, [resolvedTheme])

  useEffect(() => {
    if (theme !== "system") return
    const mql = window.matchMedia(MEDIA_QUERY)
    const onChange = () => {
      // Force a re-resolve by re-setting "system" -- cheap, and the effect
      // above only touches the DOM class when resolvedTheme actually flips.
      document.documentElement.classList.toggle("dark", systemPrefersDark())
    }
    mql.addEventListener("change", onChange)
    return () => mql.removeEventListener("change", onChange)
  }, [theme])

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t)
    try {
      localStorage.setItem(STORAGE_KEY, t)
    } catch {
      // best-effort persistence only
    }
  }, [])

  const value = useMemo(() => ({ theme, resolvedTheme, setTheme }), [theme, resolvedTheme, setTheme])
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
