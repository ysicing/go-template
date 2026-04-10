import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";
import { AdminPage } from "@/pages/admin";
import { HomePage } from "@/pages/home";
import { LoginPage } from "@/pages/login";
import { ProfilePage } from "@/pages/profile";
import { SetupPage } from "@/pages/setup";

const apiMocks = vi.hoisted(() => ({
  fetchCurrentUser: vi.fn(),
  installSystem: vi.fn(),
  login: vi.fn()
}));

vi.mock("../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../lib/api")>("../../lib/api");

  return {
    ...actual,
    fetchCurrentUser: apiMocks.fetchCurrentUser,
    installSystem: apiMocks.installSystem,
    login: apiMocks.login
  };
});

describe("page i18n", () => {
  beforeEach(async () => {
    apiMocks.fetchCurrentUser.mockReset();
    apiMocks.installSystem.mockReset();
    apiMocks.login.mockReset();
    apiMocks.fetchCurrentUser.mockResolvedValue({
      email: "admin@example.com",
      id: 1,
      role: "admin",
      username: "admin"
    });
    await i18n.changeLanguage("en-US");
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("zh-CN");
  });

  it("renders english copy for core pages", async () => {
    render(
      <AppProviders>
        <>
          <HomePage />
          <AdminPage />
          <MemoryRouter>
            <LoginPage />
            <SetupPage />
          </MemoryRouter>
          <ProfilePage />
        </>
      </AppProviders>
    );

    expect(screen.getByText("A modular Go full-stack starter for installable and embedded delivery")).toBeInTheDocument();
    expect(screen.getByText("Fiber v3 + GORM + shadcn/ui + Taskfile + Docker + GitHub Actions, ready for subsystem-oriented development")).toBeInTheDocument();
    expect(screen.getByText("Admin Console")).toBeInTheDocument();
    expect(screen.getByText("Bootstrap shell for admin subsystems")).toBeInTheDocument();
    expect(screen.getByText("Continue extending user management, audit logs, system monitoring, and more here.")).toBeInTheDocument();
    expect(screen.getByText("Username or email + password")).toBeInTheDocument();
    expect(screen.getByText("Setup Wizard")).toBeInTheDocument();
    expect(screen.getByText("Complete setup in three sections: database, cache, and admin.")).toBeInTheDocument();
    expect(screen.getByText("Database")).toBeInTheDocument();
    expect(screen.getByText("Cache")).toBeInTheDocument();
    expect(screen.getByText("Administrator")).toBeInTheDocument();
    expect(screen.queryByText("A first-run setup wizard similar to Gitea")).not.toBeInTheDocument();
    expect(screen.queryByText("JWT Secret")).not.toBeInTheDocument();
    expect(screen.getByText("Profile Summary")).toBeInTheDocument();
    expect(await screen.findByText("Signed-in account overview")).toBeInTheDocument();
    expect(screen.getByText(/Username:/)).toBeInTheDocument();
    expect(screen.getByText(/Email:/)).toBeInTheDocument();
    expect(screen.getByText(/Role:/)).toBeInTheDocument();
  });

  it("renders english fallback errors for login and setup", async () => {
    apiMocks.login.mockRejectedValue({ code: "UNKNOWN" });
    apiMocks.installSystem.mockRejectedValue({ code: "UNKNOWN" });

    render(
      <AppProviders>
        <MemoryRouter>
          <>
            <LoginPage />
            <SetupPage />
          </>
        </MemoryRouter>
      </AppProviders>
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Login" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Install" }));

    expect(await screen.findByText("Login failed")).toBeInTheDocument();
    expect(await screen.findByText("Initialization failed")).toBeInTheDocument();
  });
});
