import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../../app/providers";
import { SystemSettingsPage } from "../../pages/system-settings-page";

const settingsApiMocks = vi.hoisted(() => ({
  fetchSystemSettings: vi.fn()
}));

vi.mock("../../api/settings", () => ({
  fetchSystemSettings: settingsApiMocks.fetchSystemSettings
}));

function renderPage() {
  return render(
    <AppProviders>
      <SystemSettingsPage />
    </AppProviders>
  );
}

describe("SystemSettingsPage", () => {
  beforeEach(() => {
    settingsApiMocks.fetchSystemSettings.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders grouped settings and uncategorized fallback", async () => {
    settingsApiMocks.fetchSystemSettings.mockResolvedValue([
      { id: 1, key: "driver", value: "postgres", group: "database" },
      { id: 2, key: "addr", value: "redis:6379", group: "cache" },
      { id: 3, key: "listen", value: "0.0.0.0:3206", group: "server" },
      { id: 4, key: "mode", value: "info", group: "" }
    ]);

    renderPage();

    expect(await screen.findByText("数据库")).toBeInTheDocument();
    expect(screen.getByText("缓存")).toBeInTheDocument();
    expect(screen.getByText("服务监听")).toBeInTheDocument();
    expect(screen.getByText("未分组")).toBeInTheDocument();
    expect(screen.getByText("driver")).toBeInTheDocument();
    expect(screen.getByText("0.0.0.0:3206")).toBeInTheDocument();
  });

  it("renders empty state when no runtime settings are available", async () => {
    settingsApiMocks.fetchSystemSettings.mockResolvedValue([]);

    renderPage();

    expect(await screen.findByText("暂未生成运行期设置")).toBeInTheDocument();
    expect(screen.getByText("完成安装向导后，这里会展示数据库、缓存、监听与日志等核心配置。"))
      .toBeInTheDocument();
  });
});
