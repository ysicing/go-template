import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AdminUser } from "../../types";
import { AppProviders } from "../../../../app/providers";
import { UserManagementPage } from "../../pages/user-management-page";

const apiMocks = vi.hoisted(() => ({
  delete: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn()
}));

vi.mock("../../../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../../../lib/api")>("../../../../lib/api");

  return {
    ...actual,
    api: {
      delete: apiMocks.delete,
      get: apiMocks.get,
      post: apiMocks.post,
      put: apiMocks.put
    }
  };
});

const baseUser: AdminUser = {
  email: "alice@example.com",
  id: 1,
  last_login_at: "2026-04-10T08:30:00.000Z",
  role: "admin",
  status: "active",
  username: "alice"
};

function buildListResponse(items: AdminUser[]) {
  return {
    data: {
      data: {
        items,
        page: 1,
        page_size: 10,
        total: items.length
      }
    }
  };
}

function renderPage() {
  return render(
    <AppProviders>
      <UserManagementPage />
    </AppProviders>
  );
}

describe("UserManagementPage", () => {
  let users: AdminUser[];
  let createErrorMessage: string | null;

  beforeEach(() => {
    users = [];
    createErrorMessage = null;

    apiMocks.delete.mockReset();
    apiMocks.get.mockReset();
    apiMocks.post.mockReset();
    apiMocks.put.mockReset();

    apiMocks.get.mockImplementation(async (url: string) => {
      if (url.startsWith("/admin/users?")) {
        return buildListResponse(users);
      }

      throw new Error(`Unhandled GET: ${url}`);
    });

    apiMocks.post.mockImplementation(async (url: string, payload?: Record<string, unknown>) => {
      if (url === "/admin/users") {
        if (createErrorMessage) {
          return Promise.reject({
            response: {
              data: {
                message: createErrorMessage
              }
            }
          });
        }

        const createdUser: AdminUser = {
          email: String(payload?.email ?? ""),
          id: users.length + 1,
          last_login_at: null,
          role: payload?.role === "admin" ? "admin" : "user",
          status: payload?.status === "disabled" ? "disabled" : "active",
          username: String(payload?.username ?? "")
        };
        users = [...users, createdUser];

        return {
          data: {
            data: {
              user: createdUser
            }
          }
        };
      }

      if (url.endsWith("/disable")) {
        const userId = Number(url.split("/")[3]);
        users = users.map((item) => (item.id === userId ? { ...item, status: "disabled" } : item));

        return {
          data: {
            data: {
              disabled: true
            }
          }
        };
      }

      if (url.endsWith("/enable")) {
        const userId = Number(url.split("/")[3]);
        users = users.map((item) => (item.id === userId ? { ...item, status: "active" } : item));

        return {
          data: {
            data: {
              enabled: true
            }
          }
        };
      }

      throw new Error(`Unhandled POST: ${url}`);
    });

    apiMocks.put.mockImplementation(async (url: string, payload?: Record<string, unknown>) => {
      const userId = Number(url.split("/")[3]);

      users = users.map((item) =>
        item.id === userId
          ? {
              ...item,
              email: String(payload?.email ?? item.email),
              role: payload?.role === "admin" ? "admin" : "user",
              status: payload?.status === "disabled" ? "disabled" : "active",
              username: String(payload?.username ?? item.username)
            }
          : item
      );

      const updatedUser = users.find((item) => item.id === userId) ?? users[0];

      return {
        data: {
          data: {
            user: updatedUser
          }
        }
      };
    });

    apiMocks.delete.mockImplementation(async (url: string) => {
      const userId = Number(url.split("/")[3]);
      users = users.filter((item) => item.id !== userId);

      return {
        data: {
          data: {
            deleted: true
          }
        }
      };
    });

    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    document.body.style.overflow = "";
  });

  it("renders user filters and table headers", async () => {
    renderPage();

    expect(screen.getByRole("textbox", { name: "搜索用户" })).toBeInTheDocument();
    expect(screen.getByText("用户名")).toBeInTheDocument();
    expect(screen.getByText("邮箱")).toBeInTheDocument();
    expect(await screen.findByText("暂无用户")).toBeInTheDocument();
  });

  it("opens create, view, and edit dialogs with basic modal behavior", async () => {
    users = [baseUser];
    renderPage();

    expect(await screen.findByText("alice@example.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "新建用户" }));

    expect(screen.getByRole("dialog", { name: "新建用户" })).toBeInTheDocument();
    expect(document.body.style.overflow).toBe("hidden");

    fireEvent.keyDown(document, { key: "Escape" });

    await waitFor(() => expect(screen.queryByRole("dialog", { name: "新建用户" })).not.toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "查看" }));

    expect(screen.getByRole("dialog", { name: "用户详情" })).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("modal-backdrop"));

    await waitFor(() => expect(screen.queryByRole("dialog", { name: "用户详情" })).not.toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "编辑" }));

    expect(screen.getByRole("dialog", { name: "编辑用户" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("alice")).toBeInTheDocument();
  });

  it("submits disable action and refetches the updated list", async () => {
    users = [baseUser];
    renderPage();

    expect(await screen.findByText("alice@example.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "停用" }));

    await waitFor(() => expect(apiMocks.post).toHaveBeenCalledWith("/admin/users/1/disable"));
    await waitFor(() => expect(screen.getByRole("button", { name: "启用" })).toBeInTheDocument());
    expect(apiMocks.get.mock.calls.filter(([url]) => String(url).startsWith("/admin/users?"))).toHaveLength(2);
  });

  it("shows api error when create user fails", async () => {
    createErrorMessage = "邮箱已存在";
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "新建用户" }));
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "new-user" } });
    fireEvent.change(screen.getByLabelText("邮箱"), { target: { value: "duplicated@example.com" } });
    fireEvent.change(screen.getByLabelText("初始密码"), { target: { value: "password123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(await screen.findByText("邮箱已存在")).toBeInTheDocument();
  });
});
