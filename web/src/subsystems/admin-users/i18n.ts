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

export function getAdminUsersErrorMessage(t: TFunction<"translation">, error: unknown, fallbackMessage: string) {
  if (typeof error === "object" && error !== null && "response" in error) {
    const response = (error as { response?: { data?: { code?: string; message?: string } } }).response;
    const code = response?.data?.code;
    if (code) {
      const localizedMessage = mapAdminUsersErrorCode(t, code);
      if (localizedMessage) {
        return localizedMessage;
      }
    }
    if (response?.data?.message) {
      return response.data.message;
    }
  }

  return error instanceof Error ? error.message : fallbackMessage;
}

function mapAdminUsersErrorCode(t: TFunction<"translation">, code: string) {
  switch (code) {
    case "CANNOT_DELETE_SELF":
      return t("admin_users_error_cannot_delete_self");
    case "CANNOT_DISABLE_SELF":
      return t("admin_users_error_cannot_disable_self");
    case "DUPLICATE_EMAIL":
      return t("admin_users_error_duplicate_email");
    case "DUPLICATE_USERNAME":
      return t("admin_users_error_duplicate_username");
    case "EMAIL_REQUIRED":
      return t("admin_users_validation_email_required");
    case "INVALID_ROLE":
      return t("admin_users_error_invalid_role");
    case "INVALID_STATUS":
      return t("admin_users_error_invalid_status");
    case "PASSWORD_CONFIRMATION_MISMATCH":
      return t("admin_users_validation_password_mismatch");
    case "PASSWORD_TOO_SHORT":
      return t("admin_users_validation_password_length");
    case "USER_NOT_FOUND":
      return t("admin_users_error_not_found");
    case "USERNAME_REQUIRED":
      return t("admin_users_validation_username_required");
    default:
      return null;
  }
}
