import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { fetchSystemSettings } from "../api/settings";
import { SettingsGroupCard } from "../components/settings-group-card";
import type { SystemSetting } from "../types";

const groupOrder = ["database", "cache", "server", "log", "default"] as const;

const groupMeta: Record<string, { title: string; description: string }> = {
  cache: { title: "缓存", description: "内存缓存或 Redis 连接相关设置。" },
  database: { title: "数据库", description: "数据库驱动、地址与连接信息。" },
  default: { title: "未分组", description: "未归类但运行期仍会读取的核心配置。" },
  log: { title: "日志", description: "启动日志级别与输出行为。" },
  server: { title: "服务监听", description: "服务监听地址、端口与基础网络参数。" }
};

function normalizeGroup(group: string) {
  const normalizedGroup = group.trim().toLowerCase();
  return normalizedGroup.length > 0 ? normalizedGroup : "default";
}

function getGroupMeta(group: string) {
  return groupMeta[group] ?? { title: group, description: "自定义配置分组。" };
}

function buildGroups(items: SystemSetting[]) {
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
      ...getGroupMeta(group),
      items: [...groupItems].sort((leftItem, rightItem) => leftItem.key.localeCompare(rightItem.key))
    }));
}

export function SystemSettingsPage() {
  const query = useQuery({
    queryKey: ["system-settings"],
    queryFn: fetchSystemSettings
  });

  const groups = useMemo(() => buildGroups(query.data ?? []), [query.data]);

  if (query.isLoading) {
    return <div className="text-sm text-muted-foreground">加载系统设置中...</div>;
  }

  if (groups.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>暂未生成运行期设置</CardTitle>
          <CardDescription>安装向导会先生成最小可运行配置，后续可继续扩展更多模块设置。</CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          完成安装向导后，这里会展示数据库、缓存、监听与日志等核心配置。
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      {groups.map((group) => (
        <SettingsGroupCard description={group.description} items={group.items} key={group.title} title={group.title} />
      ))}
    </div>
  );
}
