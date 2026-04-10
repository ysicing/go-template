import type { LucideIcon } from "lucide-react"
import { Home, Settings, UserCircle, Users } from "lucide-react"

import { hasAnyAdminPermission } from "@/lib/permissions"
import type { User } from "@/stores/auth"

export type ConsoleModuleID = "home" | "account" | "admin"

export type ConsoleNavItem = {
  key: string
  labelKey: string
  icon: LucideIcon
  to: string
  matches?: string[]
}

export type ConsoleNavSection = {
  key: string
  titleKey?: string
  items: ConsoleNavItem[]
}

export type ConsoleModule = {
  id: ConsoleModuleID
  labelKey: string
  to: string
  isVisible: (user: User | null) => boolean
  sections: ConsoleNavSection[]
}

const alwaysVisible = () => true

const consoleModules: ConsoleModule[] = [
  {
    id: "home",
    labelKey: "console_module_home",
    to: "/",
    isVisible: alwaysVisible,
    sections: [
      {
        key: "home",
        items: [{ key: "home", labelKey: "console_item_home", icon: Home, to: "/", matches: ["/"] }],
      },
    ],
  },
  {
    id: "account",
    labelKey: "console_module_account",
    to: "/account/profile",
    isVisible: alwaysVisible,
    sections: [
      {
        key: "account",
        items: [{ key: "profile", labelKey: "profile", icon: UserCircle, to: "/account/profile", matches: ["/account/profile", "/profile"] }],
      },
    ],
  },
  {
    id: "admin",
    labelKey: "console_module_admin",
    to: "/admin/users",
    isVisible: hasAnyAdminPermission,
    sections: [
      {
        key: "admin-core",
        titleKey: "console_section_admin",
        items: [
          { key: "admin-users", labelKey: "admin_users", icon: Users, to: "/admin/users", matches: ["/admin/users"] },
          { key: "admin-settings", labelKey: "settings", icon: Settings, to: "/admin/settings", matches: ["/admin/settings"] },
        ],
      },
    ],
  },
]

export function getConsoleModules(user: User | null): ConsoleModule[] {
  return consoleModules.filter((module) => module.isVisible(user))
}

export function getConsoleModuleByPathname(pathname: string): ConsoleModuleID {
  if (pathname.startsWith("/admin")) return "admin"
  if (pathname.startsWith("/account") || pathname.startsWith("/profile")) return "account"
  return "home"
}

export function getConsoleCurrentModule(pathname: string, user: User | null): ConsoleModule {
  const moduleID = getConsoleModuleByPathname(pathname)
  const visibleModules = getConsoleModules(user)
  return visibleModules.find((module) => module.id === moduleID) ?? visibleModules[0] ?? consoleModules[0]
}

export function getConsoleSidebarSections(moduleID: ConsoleModuleID, user: User | null): ConsoleNavSection[] {
  const module = consoleModules.find((item) => item.id === moduleID)
  if (!module || !module.isVisible(user)) return []
  return module.sections
}

export function getConsoleModuleEntry(moduleID: ConsoleModuleID, user: User | null): string | null {
  const module = consoleModules.find((item) => item.id === moduleID)
  if (!module || !module.isVisible(user)) return null
  const firstItem = module.sections.flatMap((section) => section.items)[0]
  return firstItem?.to ?? module.to
}

export function isConsoleNavItemActive(item: ConsoleNavItem, pathname: string): boolean {
  const matchers = item.matches ?? [item.to]
  return matchers.some((match) => {
    if (match === "/") return pathname === "/"
    return pathname === match || pathname.startsWith(`${match}/`)
  })
}
