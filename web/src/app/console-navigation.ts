import type { LucideIcon } from "lucide-react";
import { Home, LayoutDashboard, Settings, UserCircle, Users } from "lucide-react";

export type ConsoleUser = {
  role: string;
};

export type ConsoleModuleID = "home" | "account" | "admin";

export interface ConsoleNavItem {
  icon: LucideIcon;
  isVisible?: (user?: ConsoleUser) => boolean;
  key: string;
  labelKey: string;
  matches?: string[];
  to: string;
}

export interface ConsoleNavSection {
  items: ConsoleNavItem[];
  key: string;
  titleKey?: string;
}

export interface ConsoleModule {
  id: ConsoleModuleID;
  isVisible: (user?: ConsoleUser) => boolean;
  labelKey: string;
  sections: ConsoleNavSection[];
  to: string;
}

const isAdmin = (user?: ConsoleUser) => user?.role === "admin";

const consoleModules: ConsoleModule[] = [
  {
    id: "home",
    isVisible: () => true,
    labelKey: "console_module_home",
    sections: [
      {
        items: [
          {
            icon: Home,
            key: "home",
            labelKey: "console_item_home",
            matches: ["/"],
            to: "/"
          }
        ],
        key: "home"
      }
    ],
    to: "/"
  },
  {
    id: "account",
    isVisible: () => true,
    labelKey: "console_module_account",
    sections: [
      {
        items: [
          {
            icon: UserCircle,
            key: "profile",
            labelKey: "profile",
            matches: ["/account/profile", "/profile"],
            to: "/account/profile"
          }
        ],
        key: "account"
      }
    ],
    to: "/account/profile"
  },
  {
    id: "admin",
    isVisible: isAdmin,
    labelKey: "console_module_admin",
    sections: [
      {
        items: [
          {
            icon: LayoutDashboard,
            key: "admin-overview",
            labelKey: "admin_overview",
            matches: ["/admin"],
            to: "/admin"
          },
          {
            icon: Users,
            key: "admin-users",
            labelKey: "admin_users",
            matches: ["/admin/users"],
            to: "/admin/users"
          },
          {
            icon: Settings,
            key: "admin-settings",
            labelKey: "settings",
            matches: ["/admin/settings"],
            to: "/admin/settings"
          }
        ],
        key: "admin-core",
        titleKey: "console_section_admin"
      }
    ],
    to: "/admin"
  }
];

export function getConsoleModules(user?: ConsoleUser) {
  return consoleModules.filter((module) => {
    if (!module.isVisible(user)) {
      return false;
    }
    if (module.id === "admin") {
      return getConsoleSidebarSections(module.id, user).length > 0;
    }
    return true;
  });
}

export function getConsoleModuleByPathname(pathname: string): ConsoleModuleID {
  if (pathname.startsWith("/admin")) {
    return "admin";
  }
  if (pathname.startsWith("/account") || pathname.startsWith("/profile")) {
    return "account";
  }
  return "home";
}

export function getConsoleCurrentModule(pathname: string, user?: ConsoleUser) {
  const visibleModules = getConsoleModules(user);
  const currentModuleID = getConsoleModuleByPathname(pathname);

  return visibleModules.find((module) => module.id === currentModuleID) ?? visibleModules[0] ?? consoleModules[0];
}

export function getConsoleSidebarSections(moduleID: ConsoleModuleID, user?: ConsoleUser) {
  const module = consoleModules.find((item) => item.id === moduleID);

  if (!module || !module.isVisible(user)) {
    return [];
  }

  return module.sections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => (item.isVisible ? item.isVisible(user) : true))
    }))
    .filter((section) => section.items.length > 0);
}

export function getConsoleModuleEntry(moduleID: ConsoleModuleID, user?: ConsoleUser) {
  const module = consoleModules.find((item) => item.id === moduleID);

  if (!module || !module.isVisible(user)) {
    return null;
  }

  const firstItem = getConsoleSidebarSections(moduleID, user)
    .flatMap((section) => section.items)
    .find((item) => item.to.length > 0);

  return firstItem?.to ?? module.to;
}

export function isConsoleNavItemActive(item: ConsoleNavItem, pathname: string) {
  const matchers = item.matches ?? [item.to];

  return matchers.some((match) => {
    if (match === "/") {
      return pathname === "/";
    }
    return pathname === match || pathname.startsWith(`${match}/`);
  });
}
