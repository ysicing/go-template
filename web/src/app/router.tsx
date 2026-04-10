import { Suspense, lazy, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from "react-router-dom";

import { fetchBuildInfo, fetchCurrentUser, fetchSetupStatus, hasAccessToken } from "@/lib/api";
import { AppShell } from "@/app/layouts/app-shell";
import { PublicLayout } from "@/app/layouts/public-layout";

const ForgotPasswordPage = lazy(() => import("@/pages/forgot-password").then((module) => ({ default: module.ForgotPasswordPage })));
const HomePage = lazy(() => import("@/pages/home").then((module) => ({ default: module.HomePage })));
const LoginPage = lazy(() => import("@/pages/login").then((module) => ({ default: module.LoginPage })));
const AdminPage = lazy(() => import("@/pages/admin").then((module) => ({ default: module.AdminPage })));
const ProfilePage = lazy(() => import("@/pages/profile").then((module) => ({ default: module.ProfilePage })));
const ResetPasswordPage = lazy(() => import("@/pages/reset-password").then((module) => ({ default: module.ResetPasswordPage })));
const SetupPage = lazy(() => import("@/pages/setup").then((module) => ({ default: module.SetupPage })));
const UserManagementPage = lazy(() =>
  import("@/subsystems/admin-users/pages/user-management-page").then((module) => ({ default: module.UserManagementPage }))
);
const SystemSettingsPage = lazy(() =>
  import("@/subsystems/system-settings/pages/system-settings-page").then((module) => ({ default: module.SystemSettingsPage }))
);

function RouteFallback() {
  const { t } = useTranslation();

  return <div className="flex min-h-[12rem] items-center justify-center text-sm text-muted-foreground">{t("app_loading")}</div>;
}

function LazyPage({ children }: { children: ReactNode }) {
  return <Suspense fallback={<RouteFallback />}>{children}</Suspense>;
}

function ApplicationRoutes() {
  const location = useLocation();
  const { t } = useTranslation();

  const setupQuery = useQuery({
    queryKey: ["setup-status"],
    queryFn: fetchSetupStatus
  });

  const setupRequired = setupQuery.data?.setup_required ?? true;
  const authEnabled = !setupRequired && hasAccessToken();
  const buildInfoQuery = useQuery({
    queryKey: ["build-info"],
    queryFn: fetchBuildInfo,
    retry: false
  });
  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: fetchCurrentUser,
    enabled: authEnabled,
    retry: false
  });

  if (setupQuery.isLoading) {
    return <div className="flex min-h-screen items-center justify-center">{t("app_loading")}</div>;
  }

  if (setupRequired && location.pathname !== "/setup") {
    return <Navigate replace to="/setup" />;
  }
  if (!setupRequired && location.pathname === "/setup") {
    return <Navigate replace to={hasAccessToken() ? "/" : "/login"} />;
  }

  return (
    <Routes>
      <Route
        element={
          <PublicLayout buildVersion={buildInfoQuery.data?.full_version} compact errorMessage={meQuery.error ? t("auth_expired") : null}>
            <Outlet />
          </PublicLayout>
        }
      >
        <Route path="/setup" element={<LazyPage><SetupPage /></LazyPage>} />
        <Route path="/login" element={!setupRequired ? <LazyPage><LoginPage /></LazyPage> : <Navigate replace to="/setup" />} />
        <Route path="/forgot-password" element={!setupRequired ? <LazyPage><ForgotPasswordPage /></LazyPage> : <Navigate replace to="/setup" />} />
        <Route path="/reset-password" element={!setupRequired ? <LazyPage><ResetPasswordPage /></LazyPage> : <Navigate replace to="/setup" />} />
      </Route>
      <Route
        element={
          <ProtectedShell
            buildVersion={buildInfoQuery.data?.full_version}
            errorMessage={meQuery.error ? t("auth_expired") : null}
            isLoading={meQuery.isLoading}
            user={meQuery.data}
          />
        }
      >
        <Route path="/" element={<LazyPage><HomePage /></LazyPage>} />
        <Route path="/profile" element={<Navigate replace to="/account/profile" />} />
        <Route path="/account/profile" element={<LazyPage><ProfilePage /></LazyPage>} />
        <Route element={<AdminRoute isLoading={meQuery.isLoading} user={meQuery.data} />}>
          <Route path="/admin" element={<LazyPage><AdminPage /></LazyPage>} />
          <Route path="/admin/users" element={<LazyPage><UserManagementPage /></LazyPage>} />
          <Route path="/admin/settings" element={<LazyPage><SystemSettingsPage /></LazyPage>} />
        </Route>
      </Route>
    </Routes>
  );
}

export function AppRouter() {
  return (
    <BrowserRouter>
      <ApplicationRoutes />
    </BrowserRouter>
  );
}

function ProtectedShell({
  buildVersion,
  errorMessage,
  isLoading,
  user
}: {
  buildVersion?: string;
  errorMessage?: string | null;
  isLoading: boolean;
  user?: { email: string; role: string; username: string };
}) {
  const { t } = useTranslation();

  if (!hasAccessToken()) {
    return <Navigate replace to="/login" />;
  }
  if (isLoading) {
    return <div className="text-sm text-muted-foreground">{t("profile_loading")}</div>;
  }
  if (!user) {
    return <Navigate replace to="/login" />;
  }

  return <AppShell buildVersion={buildVersion} errorMessage={errorMessage} user={user} />;
}

function AdminRoute({
  isLoading,
  user
}: {
  isLoading: boolean;
  user?: { role: string };
}) {
  const { t } = useTranslation();

  if (!hasAccessToken()) {
    return <Navigate replace to="/login" />;
  }
  if (isLoading) {
    return <div className="text-sm text-muted-foreground">{t("profile_loading")}</div>;
  }
  if (!user) {
    return <Navigate replace to="/login" />;
  }
  if (user.role !== "admin") {
    return <Navigate replace to="/" />;
  }
  return <Outlet />;
}
