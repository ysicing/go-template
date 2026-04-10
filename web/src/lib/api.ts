import axios from "axios";

import type { InstallFormValues } from "@/pages/setup";

export type UserInfo = {
  id: number;
  username: string;
  email: string;
  role: "user" | "admin";
};

export type BuildInfo = {
  version: string;
  commit: string;
  build_time: string;
  full_version: string;
};

const storageKeys = {
  accessToken: "app.auth.access",
  refreshToken: "app.auth.refresh"
};

export const api = axios.create({
  baseURL: "/api"
});

api.interceptors.request.use((request) => {
  const token = localStorage.getItem(storageKeys.accessToken);
  if (token) {
    request.headers.Authorization = `Bearer ${token}`;
  }
  return request;
});

export function saveTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem(storageKeys.accessToken, accessToken);
  localStorage.setItem(storageKeys.refreshToken, refreshToken);
}

export function clearTokens() {
  localStorage.removeItem(storageKeys.accessToken);
  localStorage.removeItem(storageKeys.refreshToken);
}

export function hasAccessToken() {
  return Boolean(localStorage.getItem(storageKeys.accessToken));
}

export async function fetchSetupStatus() {
  const response = await api.get("/setup/status");
  return response.data.data as { setup_required: boolean };
}

export async function installSystem(payload: InstallFormValues) {
  const response = await api.post("/setup/install", payload);
  return response.data.data as { installed: boolean };
}

export async function login(payload: { identifier: string; password: string }) {
  const response = await api.post("/auth/login", payload);
  const data = response.data.data as {
    user: UserInfo;
    token: { access_token: string; refresh_token: string };
  };
  saveTokens(data.token.access_token, data.token.refresh_token);
  return data.user;
}

export async function fetchCurrentUser() {
  const response = await api.get("/auth/me");
  return response.data.data.user as UserInfo;
}

export async function fetchSettings() {
  const response = await api.get("/system/settings");
  return response.data.data.items as Array<{ id: number; key: string; value: string; group: string }>;
}

export async function fetchBuildInfo() {
  const response = await api.get("/system/version");
  return response.data.data as BuildInfo;
}

export async function changePassword(payload: {
  old_password: string;
  new_password: string;
  confirm_new_password: string;
}) {
  const response = await api.post("/auth/change-password", payload);
  return response.data.data as { changed: boolean };
}
