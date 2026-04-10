import { useTranslation } from "react-i18next";

import type { NavigationItem } from "../shared/navigation/types";
import { getAdminUsersNavigationItems } from "../subsystems/admin-users/navigation";
import { getSystemSettingsNavigationItems } from "../subsystems/system-settings/navigation";

export function useAdminNavigation(): NavigationItem[] {
  const { t } = useTranslation();

  return [
    {
      label: t("admin_overview"),
      to: "/admin"
    },
    ...getAdminUsersNavigationItems(t),
    ...getSystemSettingsNavigationItems(t)
  ];
}
