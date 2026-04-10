import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";

import type { SystemSetting } from "../types";

export interface SettingsGroupCardProps {
  title: string;
  description: string;
  items: SystemSetting[];
}

export function SettingsGroupCard({ title, description, items }: SettingsGroupCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.map((item) => (
          <div className="rounded-lg border border-border bg-background p-3" key={item.id}>
            <div className="text-sm font-medium">{item.key}</div>
            <div className="mt-1 text-sm text-muted-foreground break-all">{item.value}</div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
