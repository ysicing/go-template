import { useQuery } from "@tanstack/react-query";
import { Globe, LogOut, MoonStar, Palette, SunMedium } from "lucide-react";
import { useTranslation } from "react-i18next";
import { BrowserRouter, Link, Navigate, Route, Routes, useLocation } from "react-router-dom";

import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { adminRouteDefinitions } from "./admin-routes";
import { adminNavigation } from "./admin-navigation";
import { AdminLayout } from "./layouts/admin-layout";
import { clearTokens, fetchBuildInfo, fetchCurrentUser, fetchSetupStatus, hasAccessToken } from "../lib/api";
import { useTheme } from "../lib/theme";
import { HomePage } from "../pages/home";
import { LoginPage } from "../pages/login";
import { ProfilePage } from "../pages/profile";
import { SetupPage } from "../pages/setup";

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
          {adminRouteDefinitions.map((route) => (
            <Route
              element={
                <AdminRoutePage
                  description={route.description}
                  isLoading={meQuery.isLoading}
                  title={route.title}
                  user={meQuery.data}
                >
                  {route.element}
                </AdminRoutePage>
              }
              key={route.path}
              path={route.path}
            />
          ))}
        </Routes>
      </main>
      <footer className="border-t border-border bg-card/60">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-3 text-xs text-muted-foreground">
          <span>Go Template</span>
          <span>{buildInfoQuery.data?.full_version ?? "version-unavailable"}</span>
        </div>
      </footer>
    </div>
  );
}

function AdminRoutePage({
  children,
  description,
  isLoading,
  title,
  user
}: {
  children: React.ReactNode;
  description: string;
  isLoading: boolean;
  title: string;
  user?: { role: string };
}) {
  return (
    <AdminRoute isLoading={isLoading} user={user}>
      <AdminLayout description={description} navigation={adminNavigation} title={title}>
        {children}
      </AdminLayout>
    </AdminRoute>
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
