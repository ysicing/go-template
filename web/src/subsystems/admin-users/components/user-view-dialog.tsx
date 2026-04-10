import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Modal } from "./modal";
import { formatAdminUserLastLoginAt, getAdminUserRoleLabel, getAdminUserStatusLabel } from "@/subsystems/admin-users/i18n";
import type { AdminUser, AdminUserStatus } from "@/subsystems/admin-users/types";

export interface UserViewDialogProps {
  open: boolean;
  user?: AdminUser | null;
  onOpenChange: (open: boolean) => void;
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
  const { i18n, t } = useTranslation();

  if (!open || !user) {
    return null;
  }

  const titleId = "user-view-dialog-title";

  return (
    <Modal labelledBy={titleId} onOpenChange={onOpenChange}>
      <Card className="w-full">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle id={titleId}>{t("admin_users_view_title")}</CardTitle>
            <CardDescription>{t("admin_users_view_description")}</CardDescription>
          </div>
          <Button size="sm" variant="ghost" onClick={() => onOpenChange(false)}>
            {t("close")}
          </Button>
        </CardHeader>
        <CardContent>
          <DetailRow label={t("admin_users_field_username")} value={user.username} />
          <DetailRow label={t("admin_users_field_email")} value={user.email} />
          <DetailRow label={t("admin_users_field_role")} value={getAdminUserRoleLabel(t, user.role)} />
          <DetailRow
            label={t("admin_users_field_status")}
            value={<Badge className={getStatusBadgeClassName(user.status)}>{getAdminUserStatusLabel(t, user.status)}</Badge>}
          />
          <DetailRow label={t("admin_users_last_login")} value={formatAdminUserLastLoginAt(t, i18n.language, user.last_login_at)} />
        </CardContent>
      </Card>
    </Modal>
  );
}
