import { MemoryRouter } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import DashboardPage from "@/pages/dashboard"
import { useAuthStore } from "@/stores/auth"
import { adminPermissions } from "@/lib/permissions"

const userMock = vi.fn()
const adminMock = vi.fn()

vi.mock("@/api/services", () => ({
  statsApi: {
    user: (...args: unknown[]) => userMock(...args),
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

    userMock.mockReset()
    adminMock.mockReset()
  })

  it("shows a setup-first control center for regular users", async () => {
    userMock.mockResolvedValue({
      data: {
        my_login_count: 0,
        app_stats: [],
      },
    })

    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    )

    expect(await screen.findByRole("heading", { name: "Identity control center" })).toBeInTheDocument()
    expect(screen.getByText("Quick access")).toBeInTheDocument()
    expect(screen.getByText("Create your first application")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Create application" })).toHaveAttribute("href", "/uauth/apps/new")
    expect(screen.getByRole("link", { name: /Profile/ })).toHaveAttribute("href", "/account/profile")
    expect(screen.getByRole("link", { name: /Points/ })).toHaveAttribute("href", "/account/points")
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

    userMock.mockResolvedValue({
      data: {
        my_login_count: 4,
        app_stats: [{ client_id: "client-1", app_name: "Portal", login_count: 4 }],
      },
    })
    adminMock.mockResolvedValue({
      data: {
        total_users: 12,
        total_clients: 3,
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
    expect(screen.getByText("Application sign-ins")).toBeInTheDocument()
    expect(screen.queryByText("Monitoring")).not.toBeInTheDocument()
    expect(adminMock).toHaveBeenCalledTimes(1)
  })
})
