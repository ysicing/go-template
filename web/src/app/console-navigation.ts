import { Home, LayoutDashboard, Settings, Shield, UserCircle, Users } from "lucide-react";

export type ConsoleUser = {
  role: string;
};

export type ConsoleModuleID = "home" | "account" | "admin";

export interface ConsoleNavItem {
  icon: typeof Home;
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
            matches: ["/profile"],
            to: "/profile"
          }
        ],
        key: "account"
      }
    ],
    to: "/profile"
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
  return consoleModules.filter((module) => module.isVisible(user));
}

export function getConsoleModuleByPathname(pathname: string): ConsoleModuleID {
  if (pathname.startsWith("/admin")) {
    return "admin";
  }
  if (pathname.startsWith("/profile")) {
    return "account";
  }
  return "home";
}

export function getConsoleCurrentModule(pathname: string, user?: ConsoleUser) {
  const visibleModules = getConsoleModules(user);
  const currentModuleID = getConsoleModuleByPathname(pathname);

  return visibleModules.find((module) => module.id === currentModuleID) ?? visibleModules[0] ?? consoleModules[0];
}

export function getConsoleSidebarSections(pathname: string, user?: ConsoleUser) {
  return getConsoleCurrentModule(pathname, user).sections;
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
