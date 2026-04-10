import { Button } from "../../../components/ui/button";
import { Badge } from "../../../shared/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../shared/ui/table";

import type { AdminUser, AdminUserRole, AdminUserStatus } from "../types";

const roleLabels: Record<AdminUserRole, string> = {
  admin: "管理员",
  user: "普通用户"
};

const statusLabels: Record<AdminUserStatus, string> = {
  active: "启用",
  disabled: "停用"
};

export interface UserTableProps {
  items: AdminUser[];
  isLoading?: boolean;
  isActionPending?: boolean;
  onView: (user: AdminUser) => void;
  onEdit: (user: AdminUser) => void;
  onToggleStatus: (user: AdminUser) => void;
  onDelete: (user: AdminUser) => void;
}

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

function EmptyState({ colSpan, message }: { colSpan: number; message: string }) {
  return (
    <TableRow>
      <TableCell className="py-8 text-center text-muted-foreground" colSpan={colSpan}>
        {message}
      </TableCell>
    </TableRow>
  );
}

export function UserTable({
  items,
  isLoading = false,
  isActionPending = false,
  onView,
  onEdit,
  onToggleStatus,
  onDelete
}: UserTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>用户名</TableHead>
          <TableHead>邮箱</TableHead>
          <TableHead>角色</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>最后登录时间</TableHead>
          <TableHead className="w-[240px]">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading ? <EmptyState colSpan={6} message="正在加载用户列表..." /> : null}
        {!isLoading && items.length === 0 ? <EmptyState colSpan={6} message="暂无用户" /> : null}
        {!isLoading
          ? items.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="font-medium">{item.username}</TableCell>
                <TableCell>{item.email}</TableCell>
                <TableCell>{roleLabels[item.role]}</TableCell>
                <TableCell>
                  <Badge className={getStatusBadgeClassName(item.status)}>{statusLabels[item.status]}</Badge>
                </TableCell>
                <TableCell>{formatLastLoginAt(item.last_login_at)}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="ghost" onClick={() => onView(item)}>
                      查看
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                    <Button size="sm" variant="ghost" disabled={isActionPending} onClick={() => onToggleStatus(item)}>
                      {item.status === "active" ? "停用" : "启用"}
                    </Button>
                    <Button size="sm" variant="ghost" disabled={isActionPending} onClick={() => onDelete(item)}>
                      删除
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          : null}
      </TableBody>
    </Table>
  );
}
