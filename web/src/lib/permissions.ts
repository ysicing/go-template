import { useAuthStore } from "@/stores/auth"

export const adminPermissions = {
  usersRead: "admin.users.read",
  usersWrite: "admin.users.write",
  clientsRead: "admin.clients.read",
  clientsWrite: "admin.clients.write",
  providersRead: "admin.providers.read",
  providersWrite: "admin.providers.write",
  settingsRead: "admin.settings.read",
  settingsWrite: "admin.settings.write",
  pointsRead: "admin.points.read",
  pointsWrite: "admin.points.write",
  loginHistoryRead: "admin.login_history.read",
  statsRead: "admin.stats.read",
} as const

export type AdminPermission = typeof adminPermissions[keyof typeof adminPermissions]

type PermissionUser = { is_admin: boolean; permissions?: string[] | string } | null | undefined

type PermissionValue = string[] | string | undefined

const allAdminPermissions = Object.values(adminPermissions)

function normalizePermissions(raw: PermissionValue): string[] {
  if (Array.isArray(raw)) {
    return raw.map((p) => p.trim()).filter((p) => p.length > 0)
  }
  if (typeof raw === "string") {
    return raw.split(",").map((p) => p.trim()).filter((p) => p.length > 0)
  }
  return []
}

export function hasPermission(user: PermissionUser, permission: AdminPermission): boolean {
  if (!user) return false
  if (user.is_admin) return true
  return normalizePermissions(user.permissions).includes(permission)
}

export function hasAnyAdminPermission(user: PermissionUser): boolean {
  if (!user) return false
  if (user.is_admin) return true
  const perms = normalizePermissions(user.permissions)
  return perms.some((perm) => allAdminPermissions.includes(perm as AdminPermission))
}

export function useHasPermission(permission: AdminPermission): boolean {
  const user = useAuthStore((s) => s.user)
  return hasPermission(user, permission)
}
