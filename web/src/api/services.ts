import api from "./client"

export const authApi = {
  config: (id?: string) => api.get("/auth/config", { params: id ? { id } : undefined }),
  login: (username: string, password: string, turnstile_token?: string, remember_me?: boolean) =>
    api.post("/auth/login", { username, password, turnstile_token, remember_me }),
  register: (username: string, email: string, password: string, turnstile_token?: string) =>
    api.post("/auth/register", { username, email, password, turnstile_token }),
  logout: () => api.post("/auth/logout"),
  oidcLogin: (id: string, username: string, password: string) =>
    api.post("/auth/oidc-login", { id, username, password }),
  oidcConsentContext: (id: string) =>
    api.get("/auth/oidc/consent", { params: { id } }),
  oidcConsentApprove: (id: string) =>
    api.post("/auth/oidc/consent/approve", { id }),
  oidcConsentDeny: (id: string) =>
    api.post("/auth/oidc/consent/deny", { id }),
  mfaVerify: (mfa_token: string, code?: string, backup_code?: string) =>
    api.post("/auth/mfa/verify", { mfa_token, code, backup_code }),
  webauthnLoginBegin: (username: string) =>
    api.post("/auth/webauthn/begin", { username }),
  webauthnLoginFinish: (webauthn_token: string, body: unknown) =>
    api.post(`/auth/webauthn/finish?webauthn_token=${webauthn_token}`, body),
  webauthnAuthBegin: (mfa_token: string) =>
    api.post("/auth/mfa/webauthn/begin", { mfa_token }),
  webauthnAuthFinish: (mfa_token: string, body: unknown) =>
    api.post(`/auth/mfa/webauthn/finish?mfa_token=${mfa_token}`, body),
  socialExchange: (code: string) =>
    api.post("/auth/social/exchange", { code }),
  confirmSocialLink: (link_token: string, payload: { password?: string; totp_code?: string }) =>
    api.post("/auth/social/confirm-link", { link_token, ...payload }),
  socialLinkWebAuthnBegin: (link_token: string) =>
    api.post("/auth/social/confirm-link/webauthn/begin", { link_token }),
  socialLinkWebAuthnFinish: (link_token: string, body: unknown) =>
    api.post(`/auth/social/confirm-link/webauthn/finish?link_token=${encodeURIComponent(link_token)}`, body),
  verifyEmail: (token: string) => api.post("/auth/verify-email", { token }),
  resendVerification: () => api.post("/auth/resend-verification"),
}

export const mfaApi = {
  status: () => api.get("/mfa/status"),
  totpSetup: () => api.post("/mfa/totp/setup"),
  totpEnable: (code: string) => api.post("/mfa/totp/enable", { code }),
  totpDisable: (password: string) => api.post("/mfa/totp/disable", { password }),
  regenerateBackupCodes: () => api.post("/mfa/backup-codes/regenerate"),
}

export const webauthnApi = {
  listCredentials: () => api.get("/mfa/webauthn/credentials"),
  registerBegin: () => api.post("/mfa/webauthn/register/begin"),
  registerFinish: (body: unknown, name?: string) =>
    api.post(`/mfa/webauthn/register/finish?name=${encodeURIComponent(name || "Passkey")}`, body),
  deleteCredential: (id: string) => api.delete(`/mfa/webauthn/credentials/${id}`),
}

export const sessionApi = {
  list: (page = 1, pageSize = 10) =>
    api.get("/sessions/", { params: { page, page_size: pageSize } }),
  revoke: (id: string) => api.delete(`/sessions/${id}`),
  revokeAll: () => api.delete("/sessions/"),
}

export const userApi = {
  getMe: () => api.get("/users/me"),
  updateMe: (data: { username?: string; email?: string; avatar_url?: string }) =>
    api.put("/users/me", data),
  changePassword: (current_password: string, new_password: string) =>
    api.put("/users/me/password", { current_password, new_password }),
  setPassword: (password: string) =>
    api.post("/users/me/set-password", { password }),
  listAuthorizedApps: (page = 1, pageSize = 10) =>
    api.get("/users/me/authorized-apps", { params: { page, page_size: pageSize } }),
  revokeAuthorizedApp: (id: string) => api.delete(`/users/me/authorized-apps/${id}`),
  listSocialAccounts: () => api.get("/users/me/social-accounts"),
  unlinkSocialAccount: (id: string) => api.delete(`/users/me/social-accounts/${id}`),
}

export const adminUserApi = {
  list: (page = 1, pageSize = 20) =>
    api.get("/admin/users", { params: { page, page_size: pageSize } }),
  get: (id: string) => api.get(`/admin/users/${id}`),
  create: (data: { username: string; email: string; is_admin: boolean }) =>
    api.post("/admin/users", data),
  delete: (id: string) => api.delete(`/admin/users/${id}`),
}

export const loginHistoryApi = {
  listMine: (page = 1, pageSize = 20) =>
    api.get("/users/me/login-history", { params: { page, page_size: pageSize } }),
  listAll: (page = 1, pageSize = 20) =>
    api.get("/admin/login-history", { params: { page, page_size: pageSize } }),
}

export const auditLogApi = {
  list: (params?: {
    page?: number
    page_size?: number
    user_id?: string
    action?: string
    resource?: string
    source?: string
    status?: string
    ip?: string
    keyword?: string
    created_from?: string
    created_to?: string
  }) => api.get("/admin/audit-logs", { params }),
}

export const adminClientApi = {
  list: (page = 1, pageSize = 20) =>
    api.get("/admin/clients", { params: { page, page_size: pageSize } }),
  get: (id: string) => api.get(`/admin/clients/${id}`),
  create: (data: Record<string, unknown>) => api.post("/admin/clients", data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/admin/clients/${id}`, data),
  delete: (id: string) => api.delete(`/admin/clients/${id}`),
}

export const adminProviderApi = {
  list: () => api.get("/admin/providers"),
  get: (id: string) => api.get(`/admin/providers/${id}`),
  create: (data: Record<string, unknown>) => api.post("/admin/providers", data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/admin/providers/${id}`, data),
  delete: (id: string) => api.delete(`/admin/providers/${id}`),
}

export const adminSettingsApi = {
  get: () => api.get("/admin/settings"),
  update: (data: Record<string, unknown>) => api.put("/admin/settings", data),
  testEmail: (to?: string) => api.post("/admin/settings/test-email", { to: to ?? "" }),
}

export const statsApi = {
  admin: () => api.get("/admin/stats"),
}

export const versionApi = {
  get: () => api.get("/version"),
}

export const pointsApi = {
  getMyPoints: () => api.get("/points"),
  getTransactions: (page = 1, pageSize = 20) =>
    api.get("/points/transactions", { params: { page, page_size: pageSize } }),
  checkIn: () => api.post("/points/checkin"),
  getCheckInStatus: (year?: number, month?: number) =>
    api.get("/points/checkin/status", { params: { year, month } }),
  spend: (amount: number, reason: string) =>
    api.post("/points/spend", { amount, reason }),
}

export const adminPointsApi = {
  adjust: (data: { user_id: string; point_type: string; amount: number; reason: string }) =>
    api.post("/admin/points/adjust", data),
  getUserPoints: (userId: string) => api.get(`/admin/points/${userId}`),
  getTransactions: (page = 1, pageSize = 20) =>
    api.get("/admin/points/transactions", { params: { page, page_size: pageSize } }),
  getLeaderboard: () => api.get("/admin/points/leaderboard"),
}
