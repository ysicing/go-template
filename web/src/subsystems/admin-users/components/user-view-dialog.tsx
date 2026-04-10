import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { Badge } from "../../../shared/ui/badge";
import { Modal } from "./modal";
import type { AdminUser, AdminUserRole, AdminUserStatus } from "../types";

export interface UserViewDialogProps {
  open: boolean;
  user?: AdminUser | null;
  onOpenChange: (open: boolean) => void;
}

const roleLabels: Record<AdminUserRole, string> = {
  admin: "管理员",
  user: "普通用户"
};

const statusLabels: Record<AdminUserStatus, string> = {
  active: "启用",
  disabled: "停用"
};

function formatLastLoginAt(value?: string | null) {
  if (!value) {
    return "从未登录";
  }

  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}

function getStatusBadgeClassName(status: AdminUserStatus) {
  return status === "active"
    ? "bg-accent/15 text-foreground ring-1 ring-inset ring-accent/20"
    : "bg-muted text-muted-foreground ring-1 ring-inset ring-border";
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-border py-3 sm:grid-cols-[120px_minmax(0,1fr)]">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="text-sm">{value}</div>
    </div>
  );
}

export function UserViewDialog({ open, user, onOpenChange }: UserViewDialogProps) {
  if (!open || !user) {
    return null;
  }

  const titleId = "user-view-dialog-title";

  return (
    <Modal labelledBy={titleId} onOpenChange={onOpenChange}>
      <Card className="w-full">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle id={titleId}>用户详情</CardTitle>
            <CardDescription>查看当前用户的基础资料与状态。</CardDescription>
          </div>
          <Button size="sm" variant="ghost" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </CardHeader>
        <CardContent>
          <DetailRow label="用户名" value={user.username} />
          <DetailRow label="邮箱" value={user.email} />
          <DetailRow label="角色" value={roleLabels[user.role]} />
          <DetailRow
            label="状态"
            value={<Badge className={getStatusBadgeClassName(user.status)}>{statusLabels[user.status]}</Badge>}
          />
          <DetailRow label="最后登录时间" value={formatLastLoginAt(user.last_login_at)} />
        </CardContent>
      </Card>
    </Modal>
  );
}
