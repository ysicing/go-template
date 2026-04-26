import i18n from "i18next"
import { initReactI18next } from "react-i18next"
import en from "./en/common.json"
import zh from "./zh/common.json"

function getBrowserStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null
  }
  const storage = window.localStorage
  if (!storage || typeof storage.getItem !== "function") {
    return null
  }
  return storage
}

const savedLang = (() => {
  try {
    const raw = getBrowserStorage()?.getItem("id-app-store")
    if (raw) {
      const store = JSON.parse(raw)
      return store?.state?.language || "zh"
    }
  } catch {
    // ignore
  }
  return "zh"
})()

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  lng: savedLang,
  fallbackLng: "en",
  interpolation: { escapeValue: false },
})

export default i18n
