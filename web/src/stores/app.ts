import { create } from "zustand"
import { persist } from "zustand/middleware"

type ThemeMode = "light" | "dark"
type Language = "en" | "zh"

// Convert hex color to oklch CSS string for shadcn/ui theme compatibility.
function hexToOklch(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16) / 255
  const g = parseInt(hex.slice(3, 5), 16) / 255
  const b = parseInt(hex.slice(5, 7), 16) / 255

  // sRGB → linear RGB
  const toLinear = (c: number) => (c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4))
  const lr = toLinear(r), lg = toLinear(g), lb = toLinear(b)

  // linear RGB → OKLab
  const l_ = Math.cbrt(0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb)
  const m_ = Math.cbrt(0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb)
  const s_ = Math.cbrt(0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb)

  const L = 0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_
  const a = 1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_
  const bOk = 0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_

  const C = Math.sqrt(a * a + bOk * bOk)
  const h = (Math.atan2(bOk, a) * 180) / Math.PI
  const hue = h < 0 ? h + 360 : h

  return `oklch(${L.toFixed(3)} ${C.toFixed(3)} ${hue.toFixed(3)})`
}

interface AppState {
  themeMode: ThemeMode
  language: Language
  primaryColor: string
  toggleTheme: () => void
  setLanguage: (lang: Language) => void
  setPrimaryColor: (color: string) => void
  applyPrimaryColor: () => void
}

export const useAppStore = create<AppState>()(
  persist(
    (set, get) => ({
      themeMode: "light",
      language: "zh",
      primaryColor: "#3b82f6",
      toggleTheme: () =>
        set((s) => ({ themeMode: s.themeMode === "light" ? "dark" : "light" })),
      setLanguage: (language) => set({ language }),
      setPrimaryColor: (color) => {
        set({ primaryColor: color })
        const oklch = hexToOklch(color)
        document.documentElement.style.setProperty("--primary", oklch)
      },
      applyPrimaryColor: () => {
        const color = get().primaryColor
        if (color && color.startsWith("#")) {
          document.documentElement.style.setProperty("--primary", hexToOklch(color))
        }
      },
    }),
    { name: "id-app-store" },
  ),
)
