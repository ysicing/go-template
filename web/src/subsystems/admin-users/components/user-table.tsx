import { useTranslation } from "react-i18next";

import { Button } from "../../../components/ui/button";
import { Badge } from "../../../shared/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../shared/ui/table";

import { formatAdminUserLastLoginAt, getAdminUserRoleLabel, getAdminUserStatusLabel } from "../i18n";
import type { AdminUser, AdminUserStatus } from "../types";

export interface UserTableProps {
  items: AdminUser[];
  isLoading?: boolean;
  isActionPending?: boolean;
  onView: (user: AdminUser) => void;
  onEdit: (user: AdminUser) => void;
  onToggleStatus: (user: AdminUser) => void;
  onDelete: (user: AdminUser) => void;
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
  const { i18n, t } = useTranslation();

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("admin_users_field_username")}</TableHead>
          <TableHead>{t("admin_users_field_email")}</TableHead>
          <TableHead>{t("admin_users_field_role")}</TableHead>
          <TableHead>{t("admin_users_field_status")}</TableHead>
          <TableHead>{t("admin_users_last_login")}</TableHead>
          <TableHead className="w-[240px]">{t("admin_users_actions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading ? <EmptyState colSpan={6} message={t("admin_users_loading")} /> : null}
        {!isLoading && items.length === 0 ? <EmptyState colSpan={6} message={t("admin_users_empty")} /> : null}
        {!isLoading
          ? items.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="font-medium">{item.username}</TableCell>
                <TableCell>{item.email}</TableCell>
                <TableCell>{getAdminUserRoleLabel(t, item.role)}</TableCell>
                <TableCell>
                  <Badge className={getStatusBadgeClassName(item.status)}>{getAdminUserStatusLabel(t, item.status)}</Badge>
                </TableCell>
                <TableCell>{formatAdminUserLastLoginAt(t, i18n.language, item.last_login_at)}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="ghost" onClick={() => onView(item)}>
                      {t("view")}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => onEdit(item)}>
                      {t("edit")}
                    </Button>
                    <Button size="sm" variant="ghost" disabled={isActionPending} onClick={() => onToggleStatus(item)}>
                      {item.status === "active" ? t("disable") : t("enable")}
                    </Button>
                    <Button size="sm" variant="ghost" disabled={isActionPending} onClick={() => onDelete(item)}>
                      {t("delete")}
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
