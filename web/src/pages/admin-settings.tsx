import { useQuery } from "@tanstack/react-query";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { fetchSettings } from "../lib/api";

export function AdminSettingsPage() {
  const query = useQuery({
    queryKey: ["settings"],
    queryFn: fetchSettings
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Settings</CardTitle>
        <CardDescription>运行期系统设置</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {(query.data ?? []).map((item) => (
          <div className="rounded-md border border-border p-3" key={item.id}>
            <div className="font-medium">{item.key}</div>
            <div className="text-muted-foreground">{item.value}</div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

