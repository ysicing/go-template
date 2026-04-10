import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { AdminPage } from "../pages/admin";
import { UserManagementPage } from "../subsystems/admin-users/pages/user-management-page";
import { SystemSettingsPage } from "../subsystems/system-settings/pages/system-settings-page";

export interface AdminRouteDefinition {
  path: string;
  title: string;
  description: string;
  element: ReactNode;
}

export function useAdminRouteDefinitions(): AdminRouteDefinition[] {
  const { t } = useTranslation();

  return [
    {
      path: "/admin",
      title: t("admin_console"),
      description: t("admin_console_description"),
      element: <AdminPage />
    },
    {
      path: "/admin/users",
      title: t("admin_users"),
      description: t("admin_users_description"),
      element: <UserManagementPage />
    },
    {
      path: "/admin/settings",
      title: t("settings"),
      description: t("admin_settings_description"),
      element: <SystemSettingsPage />
    }
  ];
}
