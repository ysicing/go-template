import { useQuery } from "@tanstack/react-query";

import { listUsers } from "../api/users";
import type { UserQueryFilters } from "../types";

export const adminUsersQueryKey = ["admin-users"] as const;

export function buildUsersSearchParams(filters: UserQueryFilters) {
  const params = new URLSearchParams();

  params.set("keyword", filters.keyword);
  params.set("role", filters.role);
  params.set("status", filters.status);
  params.set("page", String(filters.page));
  params.set("page_size", String(filters.pageSize));

  return params;
}

export function useUsers(filters: UserQueryFilters) {
  return useQuery({
    queryKey: [...adminUsersQueryKey, filters.keyword, filters.role, filters.status, filters.page, filters.pageSize],
    queryFn: () => listUsers(buildUsersSearchParams(filters))
  });
}
