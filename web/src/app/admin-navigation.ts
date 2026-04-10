import type { NavigationItem } from "../shared/navigation/types";
import { adminUsersNavigationItems } from "../subsystems/admin-users/navigation";

const adminOverviewNavigationItem: NavigationItem = {
  label: "后台概览",
  to: "/admin"
};

const adminSettingsNavigationItem: NavigationItem = {
  label: "系统设置",
  to: "/admin/settings"
};

export const adminNavigation: NavigationItem[] = [
  adminOverviewNavigationItem,
  ...adminUsersNavigationItems,
  adminSettingsNavigationItem
];
