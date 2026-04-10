import type { NavigationItem } from "../shared/navigation/types";
import { adminUsersNavigationItems } from "../subsystems/admin-users/navigation";
import { systemSettingsNavigationItems } from "../subsystems/system-settings/navigation";

const adminOverviewNavigationItem: NavigationItem = {
  label: "后台概览",
  to: "/admin"
};

export const adminNavigation: NavigationItem[] = [
  adminOverviewNavigationItem,
  ...adminUsersNavigationItems,
  ...systemSettingsNavigationItems
];
