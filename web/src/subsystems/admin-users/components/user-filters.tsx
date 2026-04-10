import { useTranslation } from "react-i18next";

import { Input } from "../../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../../components/ui/select";

import type { AdminUserRole, AdminUserStatus } from "../types";

export interface UserFiltersProps {
  keyword: string;
  role: AdminUserRole | "";
  status: AdminUserStatus | "";
  onKeywordChange: (value: string) => void;
  onRoleChange: (value: AdminUserRole | "") => void;
  onStatusChange: (value: AdminUserStatus | "") => void;
}

export function UserFilters({
  keyword,
  role,
  status,
  onKeywordChange,
  onRoleChange,
  onStatusChange
}: UserFiltersProps) {
  const { t } = useTranslation();
  const allRolesValue = "__all_roles__";
  const allStatusesValue = "__all_statuses__";

  return (
    <div className="grid gap-3 md:grid-cols-3">
      <Input
        aria-label={t("admin_users_search_label")}
        placeholder={t("admin_users_search_placeholder")}
        value={keyword}
        onChange={(event) => onKeywordChange(event.target.value)}
      />
      <Select value={role || allRolesValue} onValueChange={(value) => onRoleChange(value === allRolesValue ? "" : (value as AdminUserRole))}>
        <SelectTrigger aria-label={t("admin_users_role_filter")}>
          <SelectValue placeholder={t("admin_users_role_all")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={allRolesValue}>{t("admin_users_role_all")}</SelectItem>
          <SelectItem value="admin">{t("admin_users_role_admin")}</SelectItem>
          <SelectItem value="user">{t("admin_users_role_user")}</SelectItem>
        </SelectContent>
      </Select>
      <Select value={status || allStatusesValue} onValueChange={(value) => onStatusChange(value === allStatusesValue ? "" : (value as AdminUserStatus))}>
        <SelectTrigger aria-label={t("admin_users_status_filter")}>
          <SelectValue placeholder={t("admin_users_status_all")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={allStatusesValue}>{t("admin_users_status_all")}</SelectItem>
          <SelectItem value="active">{t("admin_users_status_active")}</SelectItem>
          <SelectItem value="disabled">{t("admin_users_status_disabled")}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
