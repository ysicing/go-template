import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { fetchSystemSettings } from "@/subsystems/system-settings/api/settings";
import { MailSettingsCard } from "@/subsystems/system-settings/components/mail-settings-card";
import { SettingsGroupCard } from "@/subsystems/system-settings/components/settings-group-card";
import type { SystemSetting } from "@/subsystems/system-settings/types";

const groupOrder = ["database", "cache", "server", "log", "default"] as const;

function normalizeGroup(group: string) {
  const normalizedGroup = group.trim().toLowerCase();
  return normalizedGroup.length > 0 ? normalizedGroup : "default";
}

function getGroupMeta(
  group: string,
  groupMeta: Record<string, { title: string; description: string }>,
  fallbackDescription: string
) {
  return groupMeta[group] ?? { title: group, description: fallbackDescription };
}

function buildGroups(
  items: SystemSetting[],
  groupMeta: Record<string, { title: string; description: string }>,
  fallbackDescription: string
) {
  const groupedItems = new Map<string, SystemSetting[]>();

  items.forEach((item) => {
    const group = normalizeGroup(item.group);
    const currentItems = groupedItems.get(group) ?? [];
    groupedItems.set(group, [...currentItems, item]);
  });

  return [...groupedItems.entries()]
    .sort(([leftGroup], [rightGroup]) => {
      const leftIndex = groupOrder.indexOf(leftGroup as (typeof groupOrder)[number]);
      const rightIndex = groupOrder.indexOf(rightGroup as (typeof groupOrder)[number]);
      const normalizedLeftIndex = leftIndex === -1 ? Number.MAX_SAFE_INTEGER : leftIndex;
      const normalizedRightIndex = rightIndex === -1 ? Number.MAX_SAFE_INTEGER : rightIndex;

      if (normalizedLeftIndex !== normalizedRightIndex) {
        return normalizedLeftIndex - normalizedRightIndex;
      }

      return leftGroup.localeCompare(rightGroup);
    })
    .map(([group, groupItems]) => ({
      ...getGroupMeta(group, groupMeta, fallbackDescription),
      items: [...groupItems].sort((leftItem, rightItem) => leftItem.key.localeCompare(rightItem.key))
    }));
}

export function SystemSettingsPage() {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: ["system-settings"],
    queryFn: fetchSystemSettings
  });

  const groupMeta = useMemo(
    () => ({
      cache: { title: t("settings_group_cache"), description: t("settings_group_cache_description") },
      database: { title: t("settings_group_database"), description: t("settings_group_database_description") },
      default: { title: t("settings_group_default"), description: t("settings_group_default_description") },
      log: { title: t("settings_group_log"), description: t("settings_group_log_description") },
      server: { title: t("settings_group_server"), description: t("settings_group_server_description") }
    }),
    [t]
  );
  const fallbackDescription = t("settings_group_custom_description");
  const groups = useMemo(() => buildGroups(query.data ?? [], groupMeta, fallbackDescription), [fallbackDescription, groupMeta, query.data]);

  if (query.isLoading) {
    return <div className="text-sm text-muted-foreground">{t("settings_loading")}</div>;
  }

  if (groups.length === 0) {
    return (
      <div className="space-y-4">
        <MailSettingsCard />
        <Card>
          <CardHeader>
            <CardTitle>{t("settings_empty_title")}</CardTitle>
            <CardDescription>{t("settings_empty_description")}</CardDescription>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">{t("settings_empty_hint")}</CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <MailSettingsCard />
      <div className="grid gap-4 xl:grid-cols-2">
        {groups.map((group) => (
          <SettingsGroupCard description={group.description} items={group.items} key={group.title} title={group.title} />
        ))}
      </div>
    </div>
  );
}
