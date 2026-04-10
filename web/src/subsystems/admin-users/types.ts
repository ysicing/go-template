export type AdminUserRole = "user" | "admin";

export type AdminUserStatus = "active" | "disabled";

export type AdminUser = {
  id: number;
  username: string;
  email: string;
  role: AdminUserRole;
  status: AdminUserStatus;
  last_login_at?: string | null;
};

export type ListUsersResponse = {
  items: AdminUser[];
  total: number;
  page: number;
  page_size: number;
};

export type UserQueryFilters = {
  keyword: string;
  role: AdminUserRole | "";
  status: AdminUserStatus | "";
  page: number;
  pageSize: number;
};

export type UserFormValues = {
  username: string;
  email: string;
  role: AdminUserRole;
  status: AdminUserStatus;
  password: string;
};

export type CreateAdminUserPayload = UserFormValues;

export type UpdateAdminUserPayload = Omit<UserFormValues, "password">;
