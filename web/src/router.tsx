import { lazy } from "react"
import { Navigate, Outlet, Route, Routes } from "react-router-dom"

import AppShell from "@/layouts/AppShell"
import { PublicLayout } from "@/layouts/PublicLayout"
import { useAuthStore } from "@/stores/auth"

const DashboardPage = lazy(() => import("@/pages/dashboard"))
const LoginPage = lazy(() => import("@/pages/login"))
const ForgotPasswordPage = lazy(() => import("@/pages/forgot-password"))
const ResetPasswordPage = lazy(() => import("@/pages/reset-password"))
const SetupPage = lazy(() => import("@/pages/setup"))
const ProfilePage = lazy(() => import("@/pages/profile"))
const UsersPage = lazy(() => import("@/pages/users"))
const SettingsPage = lazy(() => import("@/pages/settings"))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, initStatus } = useAuthStore()
  if (initStatus === "setup_required") {
    return <Navigate replace to="/setup" />
  }
  if (initStatus === "pending") {
    return <div className="text-sm text-muted-foreground">正在加载个人信息...</div>
  }
  if (!user) {
    return <Navigate replace to="/login" />
  }
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { user } = useAuthStore()
  if (!user || user.role !== "admin") {
    return <Navigate replace to="/" />
  }
  return <>{children}</>
}

export default function AppRouter() {
  const { initStatus } = useAuthStore()

  return (
    <Routes>
      <Route
        element={
          <PublicLayout>
            <Outlet />
          </PublicLayout>
        }
      >
        <Route path="/setup" element={<SetupPage />} />
        <Route path="/login" element={initStatus === "setup_required" ? <Navigate replace to="/setup" /> : <LoginPage />} />
        <Route path="/forgot-password" element={initStatus === "setup_required" ? <Navigate replace to="/setup" /> : <ForgotPasswordPage />} />
        <Route path="/reset-password" element={initStatus === "setup_required" ? <Navigate replace to="/setup" /> : <ResetPasswordPage />} />
      </Route>
      <Route
        path="/"
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="profile" element={<Navigate replace to="/account/profile" />} />
        <Route path="account">
          <Route index element={<Navigate replace to="profile" />} />
          <Route path="profile" element={<ProfilePage />} />
        </Route>
        <Route
          path="admin"
          element={
            <RequireAdmin>
              <Outlet />
            </RequireAdmin>
          }
        >
          <Route index element={<Navigate replace to="users" />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate replace to={initStatus === "setup_required" ? "/setup" : "/"} />} />
    </Routes>
  )
}
