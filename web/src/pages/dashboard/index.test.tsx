import { MemoryRouter } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import DashboardPage from "@/pages/dashboard"
import { useAuthStore } from "@/stores/auth"
import { adminPermissions } from "@/lib/permissions"

const adminMock = vi.fn()

vi.mock("@/api/services", () => ({
  statsApi: {
    admin: (...args: unknown[]) => adminMock(...args),
  },
}))

describe("DashboardPage", () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: {
        id: "user-1",
        username: "alice",
        email: "alice@example.com",
        is_admin: false,
        permissions: [],
        email_verified: true,
      },
      initStatus: "ready",
    })

    adminMock.mockReset()
  })

  it("shows account-focused quick access for regular users", async () => {
    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    )

    expect(await screen.findByRole("heading", { name: "Identity control center" })).toBeInTheDocument()
    expect(screen.getAllByText("Quick access").length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /Profile/ }).some((node) => node.getAttribute("href") === "/account/profile")).toBe(true)
    expect(screen.getAllByRole("link", { name: /Points/ }).some((node) => node.getAttribute("href") === "/account/points")).toBe(true)
    expect(screen.queryByRole("link", { name: "Create application" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: /Applications/ })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: /Quotes/ })).not.toBeInTheDocument()
    expect(screen.queryByText("Organizations")).not.toBeInTheDocument()
    expect(screen.queryByText("Service Accounts")).not.toBeInTheDocument()

    await waitFor(() => {
      expect(adminMock).not.toHaveBeenCalled()
    })
  })

  it("shows a platform snapshot for admins with stats access", async () => {
    useAuthStore.setState({
      user: {
        id: "user-1",
        username: "alice",
        email: "alice@example.com",
        is_admin: false,
        permissions: [adminPermissions.statsRead],
        email_verified: true,
      },
      initStatus: "ready",
    })

    adminMock.mockResolvedValue({
      data: {
        total_users: 12,
        total_logins: 88,
        today_logins: 7,
      },
    })

    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    )

    expect(await screen.findByText("Platform snapshot")).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: /Admin/ })).not.toBeInTheDocument()
    expect(screen.getByText("Total users")).toBeInTheDocument()
    expect(adminMock).toHaveBeenCalledTimes(1)
  })
})
