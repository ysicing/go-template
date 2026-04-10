import type { TFunction } from "i18next";

import type { NavigationItem } from "@/shared/navigation/types";

export function getSystemSettingsNavigationItems(t: TFunction<"translation">): NavigationItem[] {
  return [{ label: t("settings"), to: "/admin/settings" }];
}
