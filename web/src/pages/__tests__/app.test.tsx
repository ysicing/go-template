import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminLayout } from "../../app/layouts/admin-layout";
import { AppRouter } from "../../app/router";
import { AppProviders } from "../../app/providers";

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
      full_version: "master-abc1234-20260410T084512Z"
    });
    window.history.pushState({}, "", "/");
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
      <MemoryRouter>
        <AdminLayout title="Users" navigation={[{ label: "用户列表", to: "/admin/users" }]}>
          <div>content</div>
        </AdminLayout>
      </MemoryRouter>
    );

    expect(screen.getByText("用户列表")).toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
  });

  it("keeps parent admin link inactive on nested routes", () => {
    render(
      <MemoryRouter initialEntries={["/admin/users"]}>
        <AdminLayout
          navigation={[
            { label: "后台概览", to: "/admin" },
            { label: "用户列表", to: "/admin/users" },
            { label: "系统设置", to: "/admin/settings" }
          ]}
          title="Users"
        >
          <div>content</div>
        </AdminLayout>
      </MemoryRouter>
    );

    expect(screen.getByRole("link", { name: "用户列表" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "后台概览" })).not.toHaveAttribute("aria-current");
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

    expect(await screen.findByText("Loading profile...")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/admin");

    currentUser.resolve({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });

    expect(await screen.findByText("Admin")).toBeInTheDocument();
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

    expect(await screen.findByRole("button", { name: /login/i })).toBeInTheDocument();
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

    expect(await screen.findByText("master-abc1234-20260410T084512Z")).toBeInTheDocument();
  });
});
