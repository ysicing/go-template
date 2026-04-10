import type { PropsWithChildren } from "react"
import { Link } from "react-router-dom"
import { Moon, Sun } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { useAppStore } from "@/stores/app"

export interface PublicLayoutProps extends PropsWithChildren {
  buildVersion?: string
  errorMessage?: string | null
}

function nextLanguage(language: string) {
  return language === "zh" ? "en" : "zh"
}

function languageLabel(language: string) {
  return language === "zh" ? "EN" : "文"
}

export function PublicLayout({ buildVersion, children, errorMessage }: PublicLayoutProps) {
  const { t, i18n } = useTranslation()
  const { themeMode, toggleTheme, language, setLanguage } = useAppStore()

  const handleLanguageChange = () => {
    const next = nextLanguage(language)
    setLanguage(next)
    void i18n.changeLanguage(next)
  }

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b border-border bg-card/80 backdrop-blur">
        <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <Link className="text-sm font-medium" to="/login">
            {t("title")}
          </Link>
          <div className="flex items-center gap-2">
            <Button className="h-9 min-w-9 rounded-full px-3 text-sm font-semibold" variant="ghost" onClick={handleLanguageChange}>
              {languageLabel(language)}
            </Button>
            <Button aria-label={t("theme_toggle")} className="h-9 w-9 rounded-full p-0" variant="ghost" onClick={toggleTheme}>
              {themeMode === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 px-4 py-8">
        {errorMessage ? <Card className="p-4 text-sm text-red-500">{errorMessage}</Card> : null}
        <div className="flex flex-1 flex-col">{children}</div>
      </main>
      <footer className="border-t border-border bg-card/60">
        <div className="flex flex-col items-center justify-center gap-1 px-4 py-3 text-center text-xs text-muted-foreground">
          <span>{t("title")} · {buildVersion ?? t("version_unavailable")}</span>
          <span>{t("footer_copyright", { year: new Date().getFullYear() })}</span>
        </div>
      </footer>
    </div>
  )
}
