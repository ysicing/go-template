import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangePasswordCard } from "../change-password-card";

const apiMocks = vi.hoisted(() => ({
  changePassword: vi.fn()
}));

vi.mock("../../../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../../../lib/api")>("../../../../lib/api");

  return {
    ...actual,
    changePassword: apiMocks.changePassword
  };
});

describe("ChangePasswordCard", () => {
  beforeEach(() => {
    apiMocks.changePassword.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("validates confirm password before submit", async () => {
    render(<ChangePasswordCard />);

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: "oldpass123" } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "newpass123" } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "different123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(screen.getByText("两次输入的新密码不一致")).toBeInTheDocument();
  });

  it("submits change password payload and shows success message", async () => {
    apiMocks.changePassword.mockResolvedValue({ changed: true });

    render(<ChangePasswordCard />);

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: "oldpass123" } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "newpass123" } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "newpass123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() =>
      expect(apiMocks.changePassword).toHaveBeenCalledWith({
        old_password: "oldpass123",
        new_password: "newpass123",
        confirm_new_password: "newpass123"
      })
    );
    expect(await screen.findByText("密码修改成功")).toBeInTheDocument();
  });

  it("uses trimmed passwords for comparison and submit payload", async () => {
    apiMocks.changePassword.mockResolvedValue({ changed: true });

    render(<ChangePasswordCard />);

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: " oldpass123 " } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: " newpass123 " } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "newpass123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() =>
      expect(apiMocks.changePassword).toHaveBeenCalledWith({
        old_password: "oldpass123",
        new_password: "newpass123",
        confirm_new_password: "newpass123"
      })
    );
  });

  it("prefers backend business message when submit fails", async () => {
    apiMocks.changePassword.mockRejectedValue(
      Object.assign(new Error("Network Error"), {
        response: {
          data: {
            message: "旧密码错误"
          }
        }
      })
    );

    render(<ChangePasswordCard />);

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: "wrongpass123" } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "newpass123" } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "newpass123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(await screen.findByText("旧密码错误")).toBeInTheDocument();
  });
});
