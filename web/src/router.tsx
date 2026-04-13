import { lazy } from "react"
import { Routes, Route, Navigate, Outlet } from "react-router-dom"

import AppShell from "@/layouts/AppShell"
import { useAuthStore } from "@/stores/auth"
import { hasAnyAdminPermission, hasPermission, adminPermissions } from "@/lib/permissions"

const LoginPage = lazy(() => import("@/pages/login"))
const ConsentPage = lazy(() => import("@/pages/consent"))
const RegisterPage = lazy(() => import("@/pages/register"))
const MfaVerifyPage = lazy(() => import("@/pages/mfa-verify"))
const LoginCallbackPage = lazy(() => import("@/pages/login/callback"))
const DashboardPage = lazy(() => import("@/pages/dashboard"))
const UsersPage = lazy(() => import("@/pages/users"))
const ClientsPage = lazy(() => import("@/pages/clients"))
const ClientEditPage = lazy(() => import("@/pages/clients/edit"))
const ProvidersPage = lazy(() => import("@/pages/providers"))
const ProviderEditPage = lazy(() => import("@/pages/providers/edit"))
const SettingsPage = lazy(() => import("@/pages/settings"))
const AdminAuditLogsPage = lazy(() => import("@/pages/admin-audit-logs"))
const ProfilePage = lazy(() => import("@/pages/profile"))
const PointsPage = lazy(() => import("@/pages/points"))
const AdminPointsPage = lazy(() => import("@/pages/admin-points"))
const VerifyEmailPage = lazy(() => import("@/pages/verify-email"))
const AuthInitErrorPage = lazy(() => import("@/pages/auth-init-error"))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, initStatus } = useAuthStore()
  // Wait for auth initialization (getMe() call) to complete
  if (initStatus === "pending") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }
  if (initStatus === "service_unavailable" || initStatus === "not_found") {
    return <AuthInitErrorPage status={initStatus} />
  }
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { user } = useAuthStore()
  if (!hasAnyAdminPermission(user)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function RequireAdminPermission({
  permission,
  children,
}: {
  permission: (typeof adminPermissions)[keyof typeof adminPermissions]
  children: React.ReactNode
}) {
  const { user } = useAuthStore()
  if (!hasPermission(user, permission)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}
function AdminSection() {
  return (
    <RequireAdmin>
      <Outlet />
    </RequireAdmin>
  )
}

export default function AppRouter() {
  const { user: userForRoutes } = useAuthStore()

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/consent" element={<ConsentPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/mfa-verify" element={<MfaVerifyPage />} />
      <Route path="/login/callback" element={<LoginCallbackPage />} />
      <Route path="/verify-email" element={<VerifyEmailPage />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="account">
          <Route index element={<Navigate to="profile" replace />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="points" element={<PointsPage />} />
        </Route>
        <Route path="admin" element={<AdminSection />}>
          <Route index element={<Navigate to="users" replace />} />
          <Route path="users" element={<RequireAdminPermission permission={adminPermissions.usersRead}><UsersPage currentUser={userForRoutes ? { id: userForRoutes.id } : undefined} /></RequireAdminPermission>} />
          <Route path="clients" element={<RequireAdminPermission permission={adminPermissions.clientsRead}><ClientsPage /></RequireAdminPermission>} />
          <Route path="clients/new" element={<RequireAdminPermission permission={adminPermissions.clientsWrite}><ClientEditPage /></RequireAdminPermission>} />
          <Route path="clients/:id" element={<RequireAdminPermission permission={adminPermissions.clientsWrite}><ClientEditPage /></RequireAdminPermission>} />
          <Route path="providers" element={<RequireAdminPermission permission={adminPermissions.providersRead}><ProvidersPage /></RequireAdminPermission>} />
          <Route path="providers/new" element={<RequireAdminPermission permission={adminPermissions.providersWrite}><ProviderEditPage /></RequireAdminPermission>} />
          <Route path="providers/:id" element={<RequireAdminPermission permission={adminPermissions.providersWrite}><ProviderEditPage /></RequireAdminPermission>} />
          <Route path="settings" element={<RequireAdminPermission permission={adminPermissions.settingsRead}><SettingsPage /></RequireAdminPermission>} />
          <Route path="audit-logs" element={<RequireAdminPermission permission={adminPermissions.loginHistoryRead}><AdminAuditLogsPage /></RequireAdminPermission>} />
          <Route path="tools/points" element={<RequireAdminPermission permission={adminPermissions.pointsRead}><AdminPointsPage /></RequireAdminPermission>} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
