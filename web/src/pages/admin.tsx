import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";

export function AdminPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Admin</CardTitle>
        <CardDescription>基础管理后台壳</CardDescription>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        在这里继续扩展用户管理、审计日志、系统监控等模块。
      </CardContent>
    </Card>
  );
}

