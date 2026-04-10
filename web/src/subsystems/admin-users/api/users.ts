import { api } from "../../../lib/api";

import type { AdminUser, CreateAdminUserPayload, ListUsersResponse, ResetAdminUserPasswordPayload, UpdateAdminUserPayload } from "../types";

export async function listUsers(params: URLSearchParams) {
  const response = await api.get(`/admin/users?${params.toString()}`);
  return response.data.data as ListUsersResponse;
}

export async function createUser(payload: CreateAdminUserPayload) {
  const response = await api.post("/admin/users", payload);
  return (response.data.data as { user: AdminUser }).user;
}

export async function updateUser(userId: number, payload: UpdateAdminUserPayload) {
  const response = await api.put(`/admin/users/${userId}`, payload);
  return (response.data.data as { user: AdminUser }).user;
}

export async function enableUser(userId: number) {
  const response = await api.post(`/admin/users/${userId}/enable`);
  return response.data.data as { enabled: boolean };
}

export async function disableUser(userId: number) {
  const response = await api.post(`/admin/users/${userId}/disable`);
  return response.data.data as { disabled: boolean };
}

export async function deleteUser(userId: number) {
  const response = await api.delete(`/admin/users/${userId}`);
  return response.data.data as { deleted: boolean };
}

export async function resetUserPassword(userId: number, payload: ResetAdminUserPasswordPayload) {
  const response = await api.post(`/admin/users/${userId}/reset-password`, payload);
  return response.data.data as { changed: boolean };
}
