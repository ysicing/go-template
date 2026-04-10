import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../../app/providers";
import i18n from "../../../../lib/i18n";
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
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  function renderCard() {
    return render(
      <AppProviders>
        <ChangePasswordCard />
      </AppProviders>
    );
  }

  it("validates confirm password before submit", async () => {
    renderCard();

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: "oldpass123" } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "newpass123" } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "different123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(screen.getByText("两次输入的新密码不一致")).toBeInTheDocument();
  });

  it("submits change password payload and shows success message", async () => {
    apiMocks.changePassword.mockResolvedValue({ changed: true });

    renderCard();

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

    renderCard();

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

    renderCard();

    fireEvent.change(screen.getByLabelText("旧密码"), { target: { value: "wrongpass123" } });
    fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "newpass123" } });
    fireEvent.change(screen.getByLabelText("确认新密码"), { target: { value: "newpass123" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(await screen.findByText("旧密码错误")).toBeInTheDocument();
  });

  it("renders english copy when language switches to en-US", async () => {
    await i18n.changeLanguage("en-US");

    renderCard();

    expect(screen.getByText("Change Password")).toBeInTheDocument();
    expect(screen.getByText("Update the sign-in password for the current account")).toBeInTheDocument();
    expect(screen.getByLabelText("Old password")).toBeInTheDocument();
    expect(screen.getByLabelText("New password")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm new password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Submit" })).toBeInTheDocument();
  });
});
