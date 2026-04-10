import type { User } from "@/stores/auth"

export const adminPermissions = {
  usersRead: "admin.users.read",
  usersWrite: "admin.users.write",
  settingsRead: "admin.settings.read",
  settingsWrite: "admin.settings.write",
} as const

export type AdminPermission = typeof adminPermissions[keyof typeof adminPermissions]

export function hasPermission(user: User | null | undefined, _permission: AdminPermission): boolean {
  if (!user) return false
  if (user.role === "admin" || user.is_admin) return true
  return false
}

export function hasAnyAdminPermission(user: User | null | undefined): boolean {
  if (!user) return false
  return user.role === "admin" || user.is_admin
}
