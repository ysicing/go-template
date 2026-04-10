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
  return (
    <div className="grid gap-3 md:grid-cols-3">
      <Input
        aria-label="搜索用户"
        placeholder="搜索用户名或邮箱"
        value={keyword}
        onChange={(event) => onKeywordChange(event.target.value)}
      />
      <Select aria-label="角色筛选" value={role} onChange={(event) => onRoleChange(event.target.value as AdminUserRole | "")}>
        <option value="">全部角色</option>
        <option value="admin">管理员</option>
        <option value="user">普通用户</option>
      </Select>
      <Select aria-label="状态筛选" value={status} onChange={(event) => onStatusChange(event.target.value as AdminUserStatus | "")}>
        <option value="">全部状态</option>
        <option value="active">启用</option>
        <option value="disabled">停用</option>
      </Select>
    </div>
  );
}
