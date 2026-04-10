import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from "react-router-dom";

import { fetchBuildInfo, fetchCurrentUser, fetchSetupStatus, hasAccessToken } from "@/lib/api";
import { AppShell } from "@/app/layouts/app-shell";
import { PublicLayout } from "@/app/layouts/public-layout";
import { ForgotPasswordPage } from "@/pages/forgot-password";
import { HomePage } from "@/pages/home";
import { LoginPage } from "@/pages/login";
import { AdminPage } from "@/pages/admin";
import { ProfilePage } from "@/pages/profile";
import { ResetPasswordPage } from "@/pages/reset-password";
import { SetupPage } from "@/pages/setup";
import { UserManagementPage } from "@/subsystems/admin-users/pages/user-management-page";
import { SystemSettingsPage } from "@/subsystems/system-settings/pages/system-settings-page";

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
        <Route path="/setup" element={<SetupPage />} />
        <Route path="/login" element={!setupRequired ? <LoginPage /> : <Navigate replace to="/setup" />} />
        <Route path="/forgot-password" element={!setupRequired ? <ForgotPasswordPage /> : <Navigate replace to="/setup" />} />
        <Route path="/reset-password" element={!setupRequired ? <ResetPasswordPage /> : <Navigate replace to="/setup" />} />
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
        <Route path="/" element={<HomePage />} />
        <Route path="/profile" element={<Navigate replace to="/account/profile" />} />
        <Route path="/account/profile" element={<ProfilePage />} />
        <Route element={<AdminRoute isLoading={meQuery.isLoading} user={meQuery.data} />}>
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/admin/users" element={<UserManagementPage />} />
          <Route path="/admin/settings" element={<SystemSettingsPage />} />
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
