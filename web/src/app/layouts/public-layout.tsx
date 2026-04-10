import type { PropsWithChildren } from "react";
import { MoonStar, SunMedium } from "lucide-react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useTheme } from "@/lib/theme";

export interface PublicLayoutProps extends PropsWithChildren {
  buildVersion?: string;
  compact?: boolean;
  errorMessage?: string | null;
}

function getLanguageToggleLabel(language: string) {
  return language === "zh-CN" ? "EN" : "文";
}

function getNextLanguage(language: string) {
  return language === "zh-CN" ? "en-US" : "zh-CN";
}

export function PublicLayout({ buildVersion, children, compact = false, errorMessage }: PublicLayoutProps) {
  const { i18n, t } = useTranslation();
  const { mode, setMode } = useTheme();

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b border-border bg-card/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <nav className="flex items-center gap-4 text-sm">
            <Link to="/">{t("title")}</Link>
          </nav>
          <div className="flex items-center gap-2">
            <Button
              className="h-9 min-w-9 rounded-full px-3 text-sm font-semibold"
              variant="ghost"
              onClick={() => i18n.changeLanguage(getNextLanguage(i18n.language))}
            >
              {getLanguageToggleLabel(i18n.language)}
            </Button>
            <Button
              aria-label={t("theme_toggle")}
              className="h-9 w-9 rounded-full p-0 text-muted-foreground hover:text-foreground"
              variant="ghost"
              onClick={() => setMode(mode === "dark" ? "light" : "dark")}
            >
              {mode === "dark" ? <SunMedium className="h-4 w-4" /> : <MoonStar className="h-4 w-4" />}
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 px-4 py-8">
        {errorMessage ? <Card className="p-4 text-sm text-red-500">{errorMessage}</Card> : null}
        <div className={compact ? "" : "flex flex-1 flex-col"}>{children}</div>
      </main>
      <footer className="border-t border-border bg-card/60">
        <div className="flex flex-col items-center justify-center gap-1 px-4 py-3 text-center text-xs text-muted-foreground">
          <span>
            {t("title")} · {buildVersion ?? t("version_unavailable")}
          </span>
          <span>{t("footer_copyright", { year: new Date().getFullYear() })}</span>
        </div>
      </footer>
    </div>
  );
}
