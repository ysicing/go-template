import { useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { ChevronLeft, ChevronRight, LogOut, MoonStar, Palette, SunMedium, UserCircle2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { getConsoleCurrentModule, getConsoleModuleEntry, getConsoleModules, getConsoleSidebarSections, isConsoleNavItemActive, type ConsoleUser } from "@/app/console-navigation";
import { clearTokens } from "@/lib/api";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";

export interface AppShellProps {
  buildVersion?: string;
  errorMessage?: string | null;
  user: ConsoleUser & { email: string; username: string };
}

function getLanguageToggleLabel(language: string) {
  return language === "zh-CN" ? "EN" : "文";
}

function getNextLanguage(language: string) {
  return language === "zh-CN" ? "en-US" : "zh-CN";
}

export function AppShell({ buildVersion, errorMessage, user }: AppShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();
  const { i18n, t } = useTranslation();
  const { accent, mode, setAccent, setMode } = useTheme();
  const modules = getConsoleModules(user);
  const currentModule = getConsoleCurrentModule(location.pathname, user);
  const sidebarSections = getConsoleSidebarSections(currentModule.id, user);

  function handleLogout() {
    clearTokens();
    navigate("/login", { replace: true });
  }

  return (
    <TooltipProvider delayDuration={0}>
      <div className="flex min-h-screen bg-background text-foreground">
      <aside className={cn("hidden border-r border-border bg-card md:flex md:flex-col", collapsed ? "md:w-16" : "md:w-64")}>
        <div className="flex h-14 items-center justify-between border-b border-border px-4">
          <Link className="truncate text-sm font-semibold tracking-tight" to="/">
            {collapsed ? "GT" : t("title")}
          </Link>
          {!collapsed ? <span className="text-[10px] text-muted-foreground">{buildVersion ?? t("version_unavailable")}</span> : null}
        </div>
        <nav aria-label={t("console_sidebar_label")} className="flex-1 space-y-4 p-3">
          {sidebarSections.map((section) => (
            <div className="space-y-1" key={section.key}>
              {!collapsed && section.titleKey ? <p className="px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground/80">{t(section.titleKey)}</p> : null}
              {section.items.map((item) => {
                const Icon = item.icon;
                const active = isConsoleNavItemActive(item, location.pathname);
                const navItem = (
                  <NavLink
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      active ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      collapsed && "justify-center px-2"
                    )}
                    key={item.key}
                    to={item.to}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    {!collapsed ? <span>{t(item.labelKey)}</span> : null}
                  </NavLink>
                );

                if (!collapsed) {
                  return navItem;
                }

                return (
                  <Tooltip key={item.key}>
                    <TooltipTrigger asChild>{navItem}</TooltipTrigger>
                    <TooltipContent side="right">{t(item.labelKey)}</TooltipContent>
                  </Tooltip>
                );
              })}
            </div>
          ))}
        </nav>
        <div className="border-t border-border p-2">
          <Button
            aria-label={collapsed ? t("console_expand_sidebar") : t("console_collapse_sidebar")}
            className="w-full"
            size="sm"
            variant="ghost"
            onClick={() => setCollapsed((value) => !value)}
          >
            {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </Button>
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="border-b border-border bg-card/90 backdrop-blur">
          <div className="flex min-h-14 flex-wrap items-center justify-between gap-3 px-4 py-3 md:px-6">
            <nav aria-label={t("console_modules_label")} className="flex flex-wrap items-center gap-2">
              {modules.map((module) => (
                <Link
                  aria-current={module.id === currentModule.id ? "page" : undefined}
                  className={cn(
                    "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                    module.id === currentModule.id ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                  )}
                  key={module.id}
                  to={getConsoleModuleEntry(module.id, user) ?? module.to}
                >
                  {t(module.labelKey)}
                </Link>
              ))}
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
                size="sm"
                variant="ghost"
                onClick={() => setMode(mode === "dark" ? "light" : "dark")}
              >
                {mode === "dark" ? <SunMedium className="h-4 w-4" /> : <MoonStar className="h-4 w-4" />}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setAccent(accent === "slate" ? "blue" : accent === "blue" ? "green" : accent === "green" ? "violet" : "slate")}
              >
                <Palette className="mr-1 h-4 w-4" />
                {t("accent")}
              </Button>
              <Separator className="hidden h-6 sm:block" orientation="vertical" />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button className="gap-2 px-2" variant="ghost">
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-sm font-semibold text-foreground">
                      {user.username.slice(0, 1).toUpperCase()}
                    </span>
                    <span className="hidden text-sm sm:inline">{user.username}</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem asChild>
                    <Link to="/account/profile">
                      <UserCircle2 className="mr-2 h-4 w-4" />
                      {t("profile")}
                    </Link>
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={handleLogout}>
                    <LogOut className="mr-2 h-4 w-4" />
                    {t("logout")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </header>
        <main className="flex-1 px-4 py-6 md:px-6">
          {errorMessage ? <Card className="mb-6 p-4 text-sm text-red-500">{errorMessage}</Card> : null}
          <Outlet />
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
      </div>
    </TooltipProvider>
  );
}
