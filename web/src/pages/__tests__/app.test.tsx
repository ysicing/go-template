import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminLayout } from "@/app/layouts/admin-layout";
import { AppRouter } from "@/app/router";
import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";

const apiMocks = vi.hoisted(() => ({
  clearTokens: vi.fn(),
  fetchCurrentUser: vi.fn(),
  fetchBuildInfo: vi.fn(),
  fetchSetupStatus: vi.fn(),
  hasAccessToken: vi.fn()
}));

vi.mock("../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../lib/api")>("../../lib/api");

  return {
    ...actual,
    clearTokens: apiMocks.clearTokens,
    fetchBuildInfo: apiMocks.fetchBuildInfo,
    fetchCurrentUser: apiMocks.fetchCurrentUser,
    fetchSetupStatus: apiMocks.fetchSetupStatus,
    hasAccessToken: apiMocks.hasAccessToken
  };
});

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });

  return { promise, resolve };
}

describe("providers", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.clearTokens.mockReset();
    apiMocks.fetchBuildInfo.mockReset();
    apiMocks.fetchCurrentUser.mockReset();
    apiMocks.fetchSetupStatus.mockReset();
    apiMocks.hasAccessToken.mockReset();
    apiMocks.fetchBuildInfo.mockResolvedValue({
      full_version: "master-abc1234-20260410"
    });
    window.localStorage.clear();
    window.history.pushState({}, "", "/");
    void i18n.changeLanguage("zh-CN");
  });

  it("renders children", () => {
    render(
      <AppProviders>
        <div>hello</div>
      </AppProviders>
    );

    expect(screen.getByText("hello")).toBeInTheDocument();
  });

  it("renders admin layout navigation area", () => {
    render(
      <AppProviders>
        <MemoryRouter>
          <AdminLayout title="用户列表" navigation={[{ label: "用户列表", to: "/admin/users" }]}>
            <div>content</div>
          </AdminLayout>
        </MemoryRouter>
      </AppProviders>
    );

    expect(screen.getByRole("heading", { name: "用户列表" })).toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "用户列表导航" })).toBeInTheDocument();
  });

  it("keeps parent admin link inactive on nested routes", () => {
    render(
      <AppProviders>
        <MemoryRouter initialEntries={["/admin/users"]}>
          <AdminLayout
            navigation={[
              { label: "后台概览", to: "/admin" },
              { label: "用户列表", to: "/admin/users" },
              { label: "系统设置", to: "/admin/settings" }
            ]}
            title="用户列表"
          >
            <div>content</div>
          </AdminLayout>
        </MemoryRouter>
      </AppProviders>
    );

    expect(screen.getByRole("link", { name: "用户列表" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "后台概览" })).not.toHaveAttribute("aria-current");
  });

  it("renders console shell navigation on admin pages", async () => {
    apiMocks.hasAccessToken.mockReturnValue(true);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockResolvedValue({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });
    window.history.pushState({}, "", "/admin/users");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByRole("navigation", { name: "控制台模块导航" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "控制台侧边导航" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "管理" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "系统设置" })).toBeInTheDocument();
  });

  it("persists language choice after toggling", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    window.history.pushState({}, "", "/login");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    const languageButton = await screen.findByRole("button", { name: "EN" });

    expect(window.localStorage.getItem("app.language")).toBe("zh-CN");
    expect(document.documentElement.lang).toBe("zh-CN");

    fireEvent.click(languageButton);

    await screen.findByRole("button", { name: "文" });
    expect(window.localStorage.getItem("app.language")).toBe("en-US");
    expect(document.documentElement.lang).toBe("en-US");
  });

  it("keeps admin route during refresh while current user is loading", async () => {
    const currentUser = createDeferred<{
      email: string;
      id: number;
      role: "admin";
      username: string;
    }>();

    apiMocks.hasAccessToken.mockReturnValue(true);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockReturnValue(currentUser.promise);
    window.history.pushState({}, "", "/admin");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByText("正在加载个人信息...")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/admin");

    currentUser.resolve({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });

    expect(await screen.findByRole("navigation", { name: "控制台模块导航" })).toBeInTheDocument();
    await waitFor(() => expect(window.location.pathname).toBe("/admin"));
  });

  it("reevaluates auth state when access token appears on rerender", async () => {
    const currentUser = createDeferred<{
      email: string;
      id: number;
      role: "admin";
      username: string;
    }>();
    let accessTokenAvailable = false;

    apiMocks.hasAccessToken.mockImplementation(() => accessTokenAvailable);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockReturnValue(currentUser.promise);
    window.history.pushState({}, "", "/login");

    const view = render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByRole("button", { name: "登录" })).toBeInTheDocument();
    expect(apiMocks.fetchCurrentUser).not.toHaveBeenCalled();

    accessTokenAvailable = true;

    view.rerender(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    await waitFor(() => expect(apiMocks.fetchCurrentUser).toHaveBeenCalledTimes(1));

    currentUser.resolve({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });
  });

  it("renders backend build version in footer", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    window.history.pushState({}, "", "/login");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByText(/master-abc1234-20260410/)).toBeInTheDocument();
    expect(screen.getByText(/© 2026 ysicing/)).toBeInTheDocument();
  });

  it("renders compact language and theme controls on setup page", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: true });
    window.history.pushState({}, "", "/setup");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByRole("button", { name: "EN" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换明暗模式" })).toBeInTheDocument();
    expect(screen.queryByText("主题色")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "主题" })).not.toBeInTheDocument();
  });

  it("allows anonymous access to forgot password page", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    window.history.pushState({}, "", "/forgot-password");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByRole("heading", { name: "找回密码" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "发送重置邮件" })).toBeInTheDocument();
  });

  it("renders localized auth expired message in zh-CN", async () => {
    apiMocks.hasAccessToken.mockReturnValue(true);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockRejectedValue(new Error("token expired"));
    window.history.pushState({}, "", "/");

    render(
      <AppProviders>
        <AppRouter />
      </AppProviders>
    );

    expect(await screen.findByText("认证已过期，请重新登录。")).toBeInTheDocument();
  });
});
