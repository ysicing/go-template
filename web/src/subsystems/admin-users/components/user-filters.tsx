import { useTranslation } from "react-i18next";

import { Input } from "../../../components/ui/input";
import { Select } from "../../../shared/ui/select";

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

  return (
    <div className="grid gap-3 md:grid-cols-3">
      <Input
        aria-label={t("admin_users_search_label")}
        placeholder={t("admin_users_search_placeholder")}
        value={keyword}
        onChange={(event) => onKeywordChange(event.target.value)}
      />
      <Select aria-label={t("admin_users_role_filter")} value={role} onChange={(event) => onRoleChange(event.target.value as AdminUserRole | "")}>
        <option value="">{t("admin_users_role_all")}</option>
        <option value="admin">{t("admin_users_role_admin")}</option>
        <option value="user">{t("admin_users_role_user")}</option>
      </Select>
      <Select aria-label={t("admin_users_status_filter")} value={status} onChange={(event) => onStatusChange(event.target.value as AdminUserStatus | "")}>
        <option value="">{t("admin_users_status_all")}</option>
        <option value="active">{t("admin_users_status_active")}</option>
        <option value="disabled">{t("admin_users_status_disabled")}</option>
      </Select>
    </div>
  );
}
