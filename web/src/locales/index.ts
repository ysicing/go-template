import i18n from "i18next"
import { initReactI18next } from "react-i18next"
import en from "./en/common.json"
import zh from "./zh/common.json"

const savedLang = (() => {
  try {
    const raw = localStorage.getItem("id-app-store")
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
