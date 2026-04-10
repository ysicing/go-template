import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";
import { SetupPage } from "@/pages/setup";

const apiMocks = vi.hoisted(() => ({
  installSystem: vi.fn()
}));

vi.mock("../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../lib/api")>("../../lib/api");

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

describe("SetupPage", () => {
  beforeEach(async () => {
    apiMocks.installSystem.mockReset();
    await i18n.changeLanguage("zh-CN");
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("zh-CN");
  });

  it("renders three setup sections with independent action area", async () => {
    renderPage();

    expect(screen.getByText("按数据库、缓存、管理员三步完成系统安装。")).toBeInTheDocument();
    expect(screen.getByText("数据库设置")).toBeInTheDocument();
    expect(screen.getByText("缓存设置")).toBeInTheDocument();
    expect(screen.getByText("管理员设置")).toBeInTheDocument();
    expect(screen.getByText("确认并初始化")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "初始化系统" })).toBeInTheDocument();
  });

  it("uses the sqlite data directory dsn by default", async () => {
    renderPage();

    expect(screen.getByDisplayValue("file:data/app.db?_pragma=foreign_keys(1)")).toBeInTheDocument();
  });

  it("validates required fields before submitting", async () => {
    renderPage();

    fireEvent.change(screen.getByLabelText("管理员用户名"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "初始化系统" }));

    expect(await screen.findAllByText("请输入管理员用户名")).toHaveLength(2);
    expect(apiMocks.installSystem).not.toHaveBeenCalled();
  });

  it("shows installing feedback while setup request is pending", async () => {
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
