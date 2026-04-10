import "@testing-library/jest-dom/vitest"
import { afterEach, beforeAll, beforeEach } from "vitest"
import { cleanup } from "@testing-library/react"
import i18n from "@/locales"

beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  })

  ;(globalThis as typeof globalThis & { ResizeObserver?: typeof ResizeObserver }).ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
})

beforeEach(async () => {
  localStorage.clear()
  await i18n.changeLanguage("en")
})

afterEach(() => {
  cleanup()
})
