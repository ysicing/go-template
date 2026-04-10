import { useEffect, useState } from "react"
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom"
import { LogOut, Moon, PanelLeft, PanelLeftClose, Sun, UserCircle } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { fetchBuildInfo, clearTokens } from "@/lib/api"
import { getConsoleCurrentModule, getConsoleModuleEntry, getConsoleModules, getConsoleSidebarSections, isConsoleNavItemActive } from "@/lib/navigation"
import { cn } from "@/lib/utils"
import { useAppStore } from "@/stores/app"
import { useAuthStore } from "@/stores/auth"

export default function AppShell() {
  const [collapsed, setCollapsed] = useState(false)
  const [buildVersion, setBuildVersion] = useState("")
  const navigate = useNavigate()
  const location = useLocation()
  const { t, i18n } = useTranslation()
  const { themeMode, toggleTheme, language, setLanguage } = useAppStore()
  const { user, logout } = useAuthStore()

  useEffect(() => {
    void fetchBuildInfo().then((data) => setBuildVersion(data.full_version)).catch(() => setBuildVersion(""))
  }, [])

  const modules = getConsoleModules(user)
  const currentModule = getConsoleCurrentModule(location.pathname, user)
  const sidebarSections = getConsoleSidebarSections(currentModule.id, user)

  const handleLogout = () => {
    clearTokens()
    logout()
    navigate("/login", { replace: true })
  }

  const handleLanguageChange = () => {
    const next = language === "zh" ? "en" : "zh"
    setLanguage(next)
    void i18n.changeLanguage(next)
  }

  return (
    <TooltipProvider>
      <div className="flex h-screen overflow-hidden bg-background">
        <aside className={cn("flex flex-col border-r bg-card transition-all duration-200", collapsed ? "w-16" : "w-64")}>
          <div className="flex h-14 items-center justify-between border-b px-4">
            <span className="font-bold text-lg tracking-tight">{collapsed ? "GT" : t("title")}</span>
            {!collapsed ? <span className="text-[10px] text-muted-foreground">{buildVersion || t("version_unavailable")}</span> : null}
          </div>
          <nav aria-label={t("console_sidebar_label")} className="flex-1 p-2">
            {sidebarSections.map((section, sectionIndex) => (
              <div key={section.key} className="space-y-1">
                {!collapsed && section.titleKey ? <p className="px-3 pb-1 pt-2 text-xs font-medium uppercase tracking-wide text-muted-foreground/80">{t(section.titleKey)}</p> : null}
                {section.items.map((item) => {
                  const Icon = item.icon
                  const active = isConsoleNavItemActive(item, location.pathname)
                  const content = (
                    <>
                      <Icon className="h-4 w-4 shrink-0" />
                      {!collapsed ? <span>{t(item.labelKey)}</span> : null}
                    </>
                  )

                  return (
                    <Tooltip key={item.key} delayDuration={0}>
                      <TooltipTrigger asChild>
                        <NavLink
                          aria-current={active ? "page" : undefined}
                          className={cn(
                            "flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                            active ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                            collapsed && "justify-center px-2",
                          )}
                          to={item.to}
                        >
                          {content}
                        </NavLink>
                      </TooltipTrigger>
                      {collapsed ? <TooltipContent side="right">{t(item.labelKey)}</TooltipContent> : null}
                    </Tooltip>
                  )
                })}
                {!collapsed && sectionIndex !== sidebarSections.length - 1 ? <Separator className="my-2" /> : null}
              </div>
            ))}
          </nav>
          <div className="border-t p-2">
            <button className="flex w-full items-center justify-center rounded-md p-2 text-muted-foreground hover:bg-accent" onClick={() => setCollapsed((value) => !value)}>
              {collapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
            </button>
          </div>
        </aside>

        <div className="flex flex-1 flex-col overflow-hidden">
          <header className="flex h-14 items-center justify-between border-b bg-card px-6">
            <nav aria-label={t("console_modules_label")} className="flex items-center gap-1">
              {modules.map((module) => (
                <Link
                  key={module.id}
                  aria-current={module.id === currentModule.id ? "page" : undefined}
                  className={cn(
                    "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                    module.id === currentModule.id ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                  )}
                  to={getConsoleModuleEntry(module.id, user) ?? module.to}
                >
                  {t(module.labelKey)}
                </Link>
              ))}
            </nav>

            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" onClick={handleLanguageChange}>
                {language === "zh" ? "EN" : "文"}
              </Button>
              <Button aria-label={t("theme_toggle")} variant="ghost" size="icon" onClick={toggleTheme}>
                {themeMode === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </Button>
              <Separator orientation="vertical" className="h-6" />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" className="gap-2 px-2">
                    <Avatar className="h-7 w-7">
                      <AvatarFallback className="text-xs">{user?.username?.[0]?.toUpperCase() ?? "A"}</AvatarFallback>
                    </Avatar>
                    <span className="text-sm">{user?.username}</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem asChild>
                    <Link to="/account/profile">
                      <UserCircle className="mr-2 h-4 w-4" />
                      {t("profile")}
                    </Link>
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={handleLogout}>
                    <LogOut className="mr-2 h-4 w-4" />
                    {t("logout")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </header>

          <main className="flex-1 overflow-auto p-6">
            <Outlet />
          </main>
          <footer className="border-t border-border bg-card/60">
            <div className="flex flex-col items-center justify-center gap-1 px-4 py-3 text-center text-xs text-muted-foreground">
              <span>{t("title")} · {buildVersion || t("version_unavailable")}</span>
              <span>{t("footer_copyright", { year: new Date().getFullYear() })}</span>
            </div>
          </footer>
        </div>
      </div>
    </TooltipProvider>
  )
}
