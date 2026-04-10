import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import App from "@/App";
import i18n from "@/locales";
import { useAuthStore } from "@/stores/auth";

const apiMocks = vi.hoisted(() => ({
  clearTokens: vi.fn(),
  fetchBuildInfo: vi.fn(),
  fetchCurrentUser: vi.fn(),
  fetchSetupStatus: vi.fn(),
  hasAccessToken: vi.fn()
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

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

describe("app router", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(async () => {
    apiMocks.clearTokens.mockReset();
    apiMocks.fetchBuildInfo.mockReset();
    apiMocks.fetchCurrentUser.mockReset();
    apiMocks.fetchSetupStatus.mockReset();
    apiMocks.hasAccessToken.mockReset();
    apiMocks.fetchBuildInfo.mockResolvedValue({
      full_version: "master-abc1234-20260410"
    });
    localStorage.clear();
    useAuthStore.setState({ initStatus: "pending", user: null });
    window.history.pushState({}, "", "/");
    await i18n.changeLanguage("zh");
  });

  it("redirects first visit to setup wizard", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: true });

    render(<App />);

    expect(await screen.findByText("安装向导")).toBeInTheDocument();
    await waitFor(() => expect(window.location.pathname).toBe("/setup"));
  });

  it("renders only core console modules for admin pages", async () => {
    apiMocks.hasAccessToken.mockReturnValue(true);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockResolvedValue({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });
    window.history.pushState({}, "", "/admin/users");

    render(<App />);

    expect(await screen.findByRole("link", { name: "管理" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "首页" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "账户" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "系统设置" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "UAuth" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "监控" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "工具" })).not.toBeInTheDocument();
  });

  it("keeps anonymous forgot password page accessible", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    window.history.pushState({}, "", "/forgot-password");

    render(<App />);

    expect(await screen.findByText("找回密码")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "发送重置邮件" })).toBeInTheDocument();
  });

  it("persists language choice after toggling on login page", async () => {
    apiMocks.hasAccessToken.mockReturnValue(false);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    window.history.pushState({}, "", "/login");

    render(<App />);

    const languageButton = await screen.findByRole("button", { name: "EN" });
    expect(document.documentElement.lang).toBe("zh");

    fireEvent.click(languageButton);

    await screen.findByRole("button", { name: "文" });
    expect(document.documentElement.lang).toBe("en");
  });

  it("keeps admin route while current user is loading", async () => {
    const currentUser = createDeferred<{
      email: string;
      id: number;
      role: "admin";
      username: string;
    }>();

    apiMocks.hasAccessToken.mockReturnValue(true);
    apiMocks.fetchSetupStatus.mockResolvedValue({ setup_required: false });
    apiMocks.fetchCurrentUser.mockReturnValue(currentUser.promise);
    window.history.pushState({}, "", "/admin/users");

    render(<App />);

    expect(window.location.pathname).toBe("/admin/users");

    currentUser.resolve({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });

    expect(await screen.findByRole("link", { name: "用户管理" })).toHaveAttribute("aria-current", "page");
  });
});
