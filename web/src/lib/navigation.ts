import type { LucideIcon } from "lucide-react"
import {
  AppWindow,
  Coins,
  History,
  Home,
  KeyRound,
  Settings,
  Share2,
  UserCircle,
  Users,
} from "lucide-react"

import { adminPermissions, hasAnyAdminPermission, hasPermission } from "@/lib/permissions"
import type { User } from "@/stores/auth"

export type ConsoleModuleID = "home" | "uauth" | "account" | "admin"

export type ConsoleNavItem = {
  key: string
  labelKey: string
  icon: LucideIcon
  to: string
  external?: true
  isVisible?: (user: User | null) => boolean
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

const canAccessUsers = (user: User | null) => hasPermission(user, adminPermissions.usersRead)
const canAccessClients = (user: User | null) => hasPermission(user, adminPermissions.clientsRead)
const canAccessProviders = (user: User | null) => hasPermission(user, adminPermissions.providersRead)
const canAccessAuditLogs = (user: User | null) => hasPermission(user, adminPermissions.loginHistoryRead)
const canAccessPoints = (user: User | null) => hasPermission(user, adminPermissions.pointsRead)
const canAccessSettings = (user: User | null) => hasPermission(user, adminPermissions.settingsRead)

export const consoleModules: ConsoleModule[] = [
  {
    id: "home",
    labelKey: "nav.modules.home",
    to: "/",
    isVisible: alwaysVisible,
    sections: [
      {
        key: "home",
        items: [
          {
            key: "home-dashboard",
            labelKey: "app.dashboard",
            icon: Home,
            to: "/",
            matches: ["/"],
          },
        ],
      },
    ],
  },
  {
    id: "uauth",
    labelKey: "nav.modules.uauth",
    to: "/uauth/apps",
    isVisible: alwaysVisible,
    sections: [
      {
        key: "uauth-core",
        items: [
          { key: "uauth-apps", labelKey: "app.apps", icon: AppWindow, to: "/uauth/apps", matches: ["/uauth/apps"] },
        ],
      },
    ],
  },
  {
    id: "account",
    labelKey: "nav.modules.account",
    to: "/account/profile",
    isVisible: alwaysVisible,
    sections: [
      {
        key: "account",
        items: [
          { key: "account-profile", labelKey: "app.profile", icon: UserCircle, to: "/account/profile", matches: ["/account/profile"] },
          { key: "account-points", labelKey: "points.title", icon: Coins, to: "/account/points", matches: ["/account/points"] },
        ],
      },
    ],
  },
  {
    id: "admin",
    labelKey: "nav.modules.admin",
    to: "/admin/users",
    isVisible: hasAnyAdminPermission,
    sections: [
      {
        key: "admin-core",
        titleKey: "nav.sections.adminCore",
        items: [
          { key: "admin-users", labelKey: "app.users", icon: Users, to: "/admin/users", isVisible: canAccessUsers, matches: ["/admin/users"] },
          { key: "admin-clients", labelKey: "app.clients", icon: KeyRound, to: "/admin/clients", isVisible: canAccessClients, matches: ["/admin/clients"] },
          { key: "admin-providers", labelKey: "app.providers", icon: Share2, to: "/admin/providers", isVisible: canAccessProviders, matches: ["/admin/providers"] },
          { key: "admin-audit-logs", labelKey: "app.adminAuditLogs", icon: History, to: "/admin/audit-logs", isVisible: canAccessAuditLogs, matches: ["/admin/audit-logs"] },
          { key: "admin-settings", labelKey: "app.settings", icon: Settings, to: "/admin/settings", isVisible: canAccessSettings, matches: ["/admin/settings"] },
        ],
      },
      {
        key: "admin-tools",
        titleKey: "nav.sections.tools",
        items: [
          { key: "admin-tools-points", labelKey: "adminPoints.title", icon: Coins, to: "/admin/tools/points", isVisible: canAccessPoints, matches: ["/admin/tools/points"] },
        ],
      },
    ],
  },
]

export function getConsoleModules(user: User | null): ConsoleModule[] {
  return consoleModules.filter((module) => {
    if (!module.isVisible(user)) {
      return false
    }
    if (module.id === "admin") {
      return getConsoleSidebarSections(module.id, user).length > 0
    }
    return true
  })
}

export function getConsoleModuleByPathname(pathname: string): ConsoleModuleID {
  if (pathname === "/") return "home"
  if (pathname.startsWith("/uauth")) return "uauth"
  if (pathname.startsWith("/account")) return "account"
  if (pathname.startsWith("/admin")) return "admin"
  return "home"
}

export function getConsoleCurrentModule(pathname: string, user: User | null): ConsoleModule {
  const moduleID = getConsoleModuleByPathname(pathname)
  const visibleModules = getConsoleModules(user)
  return visibleModules.find((module) => module.id === moduleID) ?? visibleModules[0] ?? consoleModules[0]
}

export function getConsoleSidebarSections(moduleID: ConsoleModuleID, user: User | null): ConsoleNavSection[] {
  const module = consoleModules.find((item) => item.id === moduleID)
  if (!module) return []
  return module.sections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => (item.isVisible ? item.isVisible(user) : true)),
    }))
    .filter((section) => section.items.length > 0)
}

export function getConsoleModuleEntry(moduleID: ConsoleModuleID, user: User | null): string | null {
  const module = consoleModules.find((item) => item.id === moduleID)
  if (!module || !module.isVisible(user)) {
    return null
  }

  const sections = getConsoleSidebarSections(moduleID, user)
  const firstItem = sections.flatMap((section) => section.items).find((item) => !item.external)
  if (firstItem) {
    return firstItem.to
  }

  if (module.id === "admin") {
    return null
  }
  return module.to
}

export function isConsoleNavItemActive(item: ConsoleNavItem, pathname: string): boolean {
  if (item.external) return false
  const matchers = item.matches ?? [item.to]
  return matchers.some((match) => {
    if (match === "/") return pathname === "/"
    return pathname === match || pathname.startsWith(`${match}/`)
  })
}

export function redirectToSameOrigin(target: string): boolean {
  const redirectURL = new URL(target, window.location.origin)
  if (redirectURL.origin !== window.location.origin) {
    return false
  }
  window.location.assign(redirectURL.toString())
  return true
}
