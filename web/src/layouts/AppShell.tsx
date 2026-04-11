import { useEffect, useState } from "react"
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom"
import { ExternalLink, Languages, LogOut, Moon, PanelLeft, PanelLeftClose, Sun, UserCircle, Coins } from "lucide-react"
import { useTranslation } from "react-i18next"

import { authApi, versionApi } from "@/api/services"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { getConsoleCurrentModule, getConsoleModuleEntry, getConsoleModules, getConsoleSidebarSections, isConsoleNavItemActive } from "@/lib/navigation"
import { useAppStore } from "@/stores/app"
import { useAuthStore } from "@/stores/auth"

export default function AppShell() {
  const [collapsed, setCollapsed] = useState(false)
  const [versionInfo, setVersionInfo] = useState({ version: "", git_commit: "", build_date: "" })
  const navigate = useNavigate()
  const location = useLocation()
  const { t, i18n } = useTranslation()
  const { themeMode, toggleTheme, language, setLanguage } = useAppStore()
  const { user, logout } = useAuthStore()

  useEffect(() => {
    versionApi.get().then((res) => setVersionInfo(res.data)).catch(() => {})
  }, [])

  const modules = getConsoleModules(user)
  const currentModule = getConsoleCurrentModule(location.pathname, user)
  const sidebarSections = getConsoleSidebarSections(currentModule.id, user)

  const handleLogout = async () => {
    try {
      await authApi.logout()
    } catch {
      // ignore logout API errors and always clear local state
    }
    logout()
    navigate("/login")
  }

  const toggleLang = () => {
    const next = language === "zh" ? "en" : "zh"
    setLanguage(next)
    i18n.changeLanguage(next)
  }

  return (
    <TooltipProvider>
      <div className="flex h-screen overflow-hidden bg-background">
        <aside
          className={cn(
            "flex flex-col border-r bg-card transition-all duration-200",
            collapsed ? "w-16" : "w-64",
          )}
        >
          <div className="flex h-14 items-center justify-between border-b px-4">
            <span className="font-bold text-lg tracking-tight">
              {collapsed ? "ID" : t("app.title")}
            </span>
            {!collapsed && versionInfo.git_commit && (
              <Tooltip delayDuration={0}>
                <TooltipTrigger asChild>
                  <span className="cursor-help text-[10px] text-muted-foreground">
                    {versionInfo.git_commit.slice(0, 7)}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="right">
                  <div className="space-y-0.5 text-xs">
                    <div>{versionInfo.build_date}</div>
                    <div>Commit: {versionInfo.git_commit}</div>
                  </div>
                </TooltipContent>
              </Tooltip>
            )}
          </div>

          <nav className="flex-1 p-2">
            {sidebarSections.map((section, sectionIndex) => (
              <div key={section.key} className="space-y-1">
                {!collapsed && section.titleKey && (
                  <p className="px-3 pb-1 pt-2 text-xs font-medium uppercase tracking-wide text-muted-foreground/80">
                    {t(section.titleKey)}
                  </p>
                )}
                {section.items.map((item) => {
                  const Icon = item.icon
                  const active = isConsoleNavItemActive(item, location.pathname)
                  const content = (
                    <>
                      <Icon className="h-4 w-4 shrink-0" />
                      {!collapsed && (
                        <span className="flex items-center gap-1">
                          {t(item.labelKey)}
                          {item.external && <ExternalLink className="h-3 w-3" />}
                        </span>
                      )}
                    </>
                  )

                  if (item.external) {
                    return (
                      <Tooltip key={item.key} delayDuration={0}>
                        <TooltipTrigger asChild>
                          <a
                            href={item.to}
                            target="_blank"
                            rel="noreferrer"
                            className={cn(
                              "flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                              "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                              collapsed && "justify-center px-2",
                            )}
                          >
                            {content}
                          </a>
                        </TooltipTrigger>
                        {collapsed && <TooltipContent side="right">{t(item.labelKey)}</TooltipContent>}
                      </Tooltip>
                    )
                  }

                  return (
                    <Tooltip key={item.key} delayDuration={0}>
                      <TooltipTrigger asChild>
                        <NavLink
                          to={item.to}
                          aria-current={active ? "page" : undefined}
                          className={cn(
                            "flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                            active
                              ? "bg-primary text-primary-foreground"
                              : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                            collapsed && "justify-center px-2",
                          )}
                        >
                          {content}
                        </NavLink>
                      </TooltipTrigger>
                      {collapsed && <TooltipContent side="right">{t(item.labelKey)}</TooltipContent>}
                    </Tooltip>
                  )
                })}
                {!collapsed && sectionIndex !== sidebarSections.length - 1 && <Separator className="my-2" />}
              </div>
            ))}
          </nav>

          <div className="border-t p-2">
            <button
              onClick={() => setCollapsed((value) => !value)}
              className="flex w-full items-center justify-center rounded-md p-2 text-muted-foreground hover:bg-accent"
            >
              {collapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
            </button>
          </div>
        </aside>

        <div className="flex flex-1 flex-col overflow-hidden">
          <header className="flex h-14 items-center justify-between border-b bg-card px-6">
            <nav className="flex items-center gap-1">
              {modules.map((module) => {
                const active = module.id === currentModule.id
                const target = getConsoleModuleEntry(module.id, user) ?? module.to
                return (
                  <Link
                    key={module.id}
                    to={target}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      active
                        ? "bg-primary text-primary-foreground"
                        : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                    )}
                  >
                    {t(module.labelKey)}
                  </Link>
                )
              })}
            </nav>

            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" onClick={toggleLang}>
                <Languages className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={toggleTheme}>
                {themeMode === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </Button>
              <Separator orientation="vertical" className="h-6" />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" className="gap-2 px-2">
                    <Avatar className="h-7 w-7">
                      <AvatarFallback className="text-xs">
                        {user?.username?.[0]?.toUpperCase() ?? "A"}
                      </AvatarFallback>
                    </Avatar>
                    <span className="text-sm">{user?.username}</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem asChild>
                    <Link to="/account/profile">
                      <UserCircle className="mr-2 h-4 w-4" />
                      {t("app.profile")}
                    </Link>
                  </DropdownMenuItem>
                  <DropdownMenuItem asChild>
                    <Link to="/account/points">
                      <Coins className="mr-2 h-4 w-4" />
                      {t("points.title")}
                    </Link>
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={handleLogout}>
                    <LogOut className="mr-2 h-4 w-4" />
                    {t("app.logout")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </header>

          <main className="flex-1 overflow-auto p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </TooltipProvider>
  )
}
