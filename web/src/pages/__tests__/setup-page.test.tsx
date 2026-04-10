import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "@/app/providers";
import { SetupPage } from "@/pages/setup";
import i18n from "@/locales";

const apiMocks = vi.hoisted(() => ({
  installSystem: vi.fn()
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    installSystem: apiMocks.installSystem
  };
});

function renderPage() {
  return render(
    <AppProviders>
      <MemoryRouter>
        <SetupPage />
      </MemoryRouter>
    </AppProviders>
  );
}

describe("setup page", () => {
  beforeEach(async () => {
    apiMocks.installSystem.mockReset();
    await i18n.changeLanguage("zh");
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("zh");
  });

  it("renders database cache admin sections", () => {
    renderPage();

    expect(screen.getByText("数据库设置")).toBeInTheDocument();
    expect(screen.getByText("缓存设置")).toBeInTheDocument();
    expect(screen.getByText("管理员设置")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "初始化系统" })).toBeInTheDocument();
  });

  it("uses sqlite data dsn by default", () => {
    renderPage();

    expect(screen.getByDisplayValue("file:data/app.db?_pragma=foreign_keys(1)")).toBeInTheDocument();
  });

  it("validates required fields before submit", async () => {
    renderPage();

    fireEvent.change(screen.getByLabelText("管理员用户名"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "初始化系统" }));

    expect(await screen.findAllByText("请输入管理员用户名")).toHaveLength(2);
    expect(apiMocks.installSystem).not.toHaveBeenCalled();
  });

  it("shows installing feedback while request is pending", async () => {
    let resolveInstall: (() => void) | undefined;
    apiMocks.installSystem.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveInstall = resolve;
      })
    );

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "初始化系统" }));

    expect(await screen.findByRole("button", { name: "校验连接并初始化中..." })).toBeDisabled();

    resolveInstall?.();

    await waitFor(() => expect(apiMocks.installSystem).toHaveBeenCalledTimes(1));
  });
});
