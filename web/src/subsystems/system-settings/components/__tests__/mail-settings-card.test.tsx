import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";
import { MailSettingsCard } from "@/subsystems/system-settings/components/mail-settings-card";

const settingsApiMocks = vi.hoisted(() => ({
  fetchMailSettings: vi.fn(),
  updateMailSettings: vi.fn()
}));

vi.mock("../../api/settings", () => ({
  fetchMailSettings: settingsApiMocks.fetchMailSettings,
  updateMailSettings: settingsApiMocks.updateMailSettings
}));

function renderCard() {
  return render(
    <AppProviders>
      <MailSettingsCard />
    </AppProviders>
  );
}

describe("MailSettingsCard", () => {
  beforeEach(() => {
    settingsApiMocks.fetchMailSettings.mockReset();
    settingsApiMocks.updateMailSettings.mockReset();
    settingsApiMocks.fetchMailSettings.mockResolvedValue({
      enabled: true,
      from: "noreply@example.com",
      password: "",
      password_set: true,
      reset_base_url: "http://127.0.0.1:3206",
      site_name: "Go Template",
      smtp_host: "smtp.example.com",
      smtp_port: 587,
      username: "mailer"
    });
    settingsApiMocks.updateMailSettings.mockImplementation(async (payload) => ({
      ...payload,
      password: "",
      password_set: true
    }));
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
    void i18n.changeLanguage("zh-CN");
  });

  it("submits updated mail settings", async () => {
    renderCard();

    expect(await screen.findByDisplayValue("smtp.example.com")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("SMTP 主机"), { target: { value: "smtp.internal.local" } });
    fireEvent.change(screen.getByLabelText("SMTP 密码"), { target: { value: "new-secret" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(settingsApiMocks.updateMailSettings).toHaveBeenCalledTimes(1));
    expect(settingsApiMocks.updateMailSettings.mock.calls[0]?.[0]).toEqual({
      enabled: true,
      from: "noreply@example.com",
      password: "new-secret",
      password_set: true,
      reset_base_url: "http://127.0.0.1:3206",
      site_name: "Go Template",
      smtp_host: "smtp.internal.local",
      smtp_port: 587,
      username: "mailer"
    });

    expect(await screen.findByText("邮件设置已保存")).toBeInTheDocument();
  });
});
