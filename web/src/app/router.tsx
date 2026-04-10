import { useQuery } from "@tanstack/react-query";
import { Globe, LogOut, MoonStar, Palette, SunMedium } from "lucide-react";
import { useTranslation } from "react-i18next";
import { BrowserRouter, Link, Navigate, Route, Routes, useLocation } from "react-router-dom";

import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { adminNavigation } from "./admin-navigation";
import { AdminLayout } from "./layouts/admin-layout";
import { clearTokens, fetchCurrentUser, fetchSetupStatus, hasAccessToken } from "../lib/api";
import { useTheme } from "../lib/theme";
import { AdminPage } from "../pages/admin";
import { AdminSettingsPage } from "../pages/admin-settings";
import { HomePage } from "../pages/home";
import { LoginPage } from "../pages/login";
import { ProfilePage } from "../pages/profile";
import { SetupPage } from "../pages/setup";
import { UserManagementPage } from "../subsystems/admin-users/pages/user-management-page";

export function AppRouter() {
  return (
    <BrowserRouter>
      <ApplicationRoutes />
    </BrowserRouter>
  );
}

function ApplicationRoutes() {
  const location = useLocation();
  const { i18n, t } = useTranslation();
  const { accent, mode, setAccent, setMode } = useTheme();

  const setupQuery = useQuery({
    queryKey: ["setup-status"],
    queryFn: fetchSetupStatus
  });

  const setupRequired = setupQuery.data?.setup_required ?? true;
  const authEnabled = !setupRequired && hasAccessToken();
  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: fetchCurrentUser,
    enabled: authEnabled,
    retry: false
  });

  if (setupQuery.isLoading) {
    return <div className="flex min-h-screen items-center justify-center">Loading...</div>;
  }

  if (setupRequired && location.pathname !== "/setup") {
    return <Navigate replace to="/setup" />;
  }
  if (!setupRequired && location.pathname === "/setup") {
    return <Navigate replace to={hasAccessToken() ? "/" : "/login"} />;
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border bg-card/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <nav className="flex items-center gap-4 text-sm">
            <Link to="/">{t("title")}</Link>
            {!setupRequired ? (
              <>
                <Link to="/profile">{t("profile")}</Link>
                <Link to="/admin">{t("admin")}</Link>
                <Link to="/admin/settings">{t("settings")}</Link>
              </>
            ) : null}
          </nav>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => i18n.changeLanguage(i18n.language === "zh-CN" ? "en-US" : "zh-CN")}>
              <Globe className="mr-1 h-4 w-4" />
              {t("language")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setMode(mode === "dark" ? "light" : "dark")}>
              {mode === "dark" ? <SunMedium className="mr-1 h-4 w-4" /> : <MoonStar className="mr-1 h-4 w-4" />}
              {t("theme")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAccent(accent === "slate" ? "blue" : accent === "blue" ? "green" : accent === "green" ? "violet" : "slate")}
            >
              <Palette className="mr-1 h-4 w-4" />
              {t("accent")}
            </Button>
            {hasAccessToken() ? (
              <Button
                size="sm"
                onClick={() => {
                  clearTokens();
                  window.location.href = "/login";
                }}
              >
                <LogOut className="mr-1 h-4 w-4" />
                {t("logout")}
              </Button>
            ) : null}
          </div>
        </div>
      </header>
      <main className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-8">
        {meQuery.error ? <Card className="p-4 text-sm text-red-500">Authentication expired, please login again.</Card> : null}
        <Routes>
          <Route path="/setup" element={<SetupPage />} />
          <Route path="/login" element={!setupRequired ? <LoginPage /> : <Navigate replace to="/setup" />} />
          <Route path="/" element={<ProtectedRoute isLoading={meQuery.isLoading} user={meQuery.data}><HomePage /></ProtectedRoute>} />
          <Route path="/profile" element={<ProtectedRoute isLoading={meQuery.isLoading} user={meQuery.data}><ProfilePage /></ProtectedRoute>} />
          <Route
            path="/admin"
            element={
              <AdminRoute isLoading={meQuery.isLoading} user={meQuery.data}>
                <AdminLayout
                  description="后台模块入口与当前系统概览。"
                  navigation={adminNavigation}
                  title="管理后台"
                >
                  <AdminPage />
                </AdminLayout>
              </AdminRoute>
            }
          />
          <Route
            path="/admin/users"
            element={
              <AdminRoute isLoading={meQuery.isLoading} user={meQuery.data}>
                <AdminLayout
                  description="查看、筛选并维护系统中的用户账号。"
                  navigation={adminNavigation}
                  title="用户管理"
                >
                  <UserManagementPage />
                </AdminLayout>
              </AdminRoute>
            }
          />
          <Route
            path="/admin/settings"
            element={
              <AdminRoute isLoading={meQuery.isLoading} user={meQuery.data}>
                <AdminLayout
                  description="运行期系统设置与后台配置。"
                  navigation={adminNavigation}
                  title="系统设置"
                >
                  <AdminSettingsPage />
                </AdminLayout>
              </AdminRoute>
            }
          />
        </Routes>
      </main>
    </div>
  );
}

function ProtectedRoute({
  children,
  isLoading,
  user
}: {
  children: React.ReactNode;
  isLoading: boolean;
  user?: { role: string };
}) {
  if (!hasAccessToken()) {
    return <Navigate replace to="/login" />;
  }
  if (isLoading) {
    return <div className="text-sm text-muted-foreground">Loading profile...</div>;
  }
  if (!user) {
    return <Navigate replace to="/login" />;
  }
  return <>{children}</>;
}

function AdminRoute({
  children,
  isLoading,
  user
}: {
  children: React.ReactNode;
  isLoading: boolean;
  user?: { role: string };
}) {
  if (!hasAccessToken()) {
    return <Navigate replace to="/login" />;
  }
  if (isLoading) {
    return <div className="text-sm text-muted-foreground">Loading profile...</div>;
  }
  if (!user) {
    return <Navigate replace to="/login" />;
  }
  if (user.role !== "admin") {
    return <Navigate replace to="/" />;
  }
  return <>{children}</>;
}
