import type { TFunction } from "i18next";

import type { AdminUserRole, AdminUserStatus } from "./types";

export function getAdminUserRoleLabel(t: TFunction<"translation">, role: AdminUserRole) {
  return role === "admin" ? t("admin_users_role_admin") : t("admin_users_role_user");
}

export function getAdminUserStatusLabel(t: TFunction<"translation">, status: AdminUserStatus) {
  return status === "active" ? t("admin_users_status_active") : t("admin_users_status_disabled");
}

export function formatAdminUserLastLoginAt(t: TFunction<"translation">, language: string, value?: string | null) {
  if (!value) {
    return t("admin_users_never_logged_in");
  }

  return new Date(value).toLocaleString(language === "zh-CN" ? "zh-CN" : "en-US", { hour12: false });
}
