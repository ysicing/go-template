import type { TFunction } from "i18next";

import type { NavigationItem } from "../../shared/navigation/types";

export function getAdminUsersNavigationItems(t: TFunction<"translation">): NavigationItem[] {
  return [{ label: t("admin_users"), to: "/admin/users" }];
}
