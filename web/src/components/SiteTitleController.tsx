import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { authApi } from "@/api/services"

export function SiteTitleController() {
  const { t, i18n } = useTranslation()

  useEffect(() => {
    let cancelled = false

    const fallbackTitle = () => t("app.title")

    const applyTitle = (siteTitle?: string) => {
      if (cancelled) {
        return
      }
      const title = siteTitle?.trim() || fallbackTitle()
      document.title = title
    }

    authApi
      .config()
      .then((res) => {
        applyTitle(typeof res.data?.site_title === "string" ? res.data.site_title : "")
      })
      .catch(() => {
        applyTitle("")
      })

    return () => {
      cancelled = true
    }
  }, [i18n.language, t])

  return null
}
