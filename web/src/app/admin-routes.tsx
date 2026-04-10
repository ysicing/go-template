import type { ReactNode } from "react";

import { AdminPage } from "../pages/admin";
import { UserManagementPage } from "../subsystems/admin-users/pages/user-management-page";
import { SystemSettingsPage } from "../subsystems/system-settings/pages/system-settings-page";

export interface AdminRouteDefinition {
  path: string;
  title: string;
  description: string;
  element: ReactNode;
}

export const adminRouteDefinitions: AdminRouteDefinition[] = [
  {
    path: "/admin",
    title: "管理后台",
    description: "后台模块入口与当前系统概览。",
    element: <AdminPage />
  },
  {
    path: "/admin/users",
    title: "用户管理",
    description: "查看、筛选并维护系统中的用户账号。",
    element: <UserManagementPage />
  },
  {
    path: "/admin/settings",
    title: "系统设置",
    description: "运行期系统设置与后台配置。",
    element: <SystemSettingsPage />
  }
];
