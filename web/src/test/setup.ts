import "@testing-library/jest-dom/vitest"
import { afterEach, beforeAll, beforeEach } from "vitest"
import { cleanup } from "@testing-library/react"
import i18n from "@/locales"

function createMemoryStorage(): Storage {
  const values = new Map<string, string>()

  return {
    get length() {
      return values.size
    },
    clear() {
      values.clear()
    },
    getItem(key: string) {
      return values.get(key) ?? null
    },
    key(index: number) {
      return Array.from(values.keys())[index] ?? null
    },
    removeItem(key: string) {
      values.delete(key)
    },
    setItem(key: string, value: string) {
      values.set(key, String(value))
    },
  }
}

beforeAll(() => {
  const storage = createMemoryStorage()

  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: storage,
  })
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: storage,
  })

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
  window.localStorage.clear()
  await i18n.changeLanguage("en")
})

afterEach(() => {
  cleanup()
})
