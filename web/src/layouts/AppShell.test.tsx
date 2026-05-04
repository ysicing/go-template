import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import AppShell from "@/layouts/AppShell"
import { adminPermissions } from "@/lib/permissions"
import { useAppStore } from "@/stores/app"
import { useAuthStore } from "@/stores/auth"

const versionGetMock = vi.fn()
const logoutMock = vi.fn()

vi.mock("@/api/services", () => ({
  versionApi: {
    get: (...args: unknown[]) => versionGetMock(...args),
  },
  authApi: {
    logout: (...args: unknown[]) => logoutMock(...args),
  },
}))

function renderShell(initialEntry: string) {
  function LocationProbe() {
    const location = useLocation()
    return <div data-testid="pathname">{location.pathname}</div>
  }

  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="*" element={<AppShell />}>
          <Route path="*" element={<LocationProbe />} />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

describe("AppShell", () => {
  beforeEach(() => {
    versionGetMock.mockReset()
    logoutMock.mockReset()
    versionGetMock.mockResolvedValue({ data: { version: "v1.0.0", git_commit: "abc1234", build_date: "2026-04-07T00:00:00Z" } })
    useAppStore.setState({ themeMode: "light", language: "en", primaryColor: "#3b82f6" })
  })

  it("falls back to home navigation for removed uauth paths", async () => {
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

    renderShell("/uauth/apps")

    expect(await screen.findByRole("combobox", { name: "Subsystem" })).toHaveTextContent("Home")
    expect(screen.getByText("v1.0.0 · abc1234")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Control Center" })).toHaveAttribute("href", "/")
    expect(screen.queryByRole("link", { name: "Applications" })).not.toBeInTheDocument()
    expect(screen.queryByText("UAuth")).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Monitoring" })).not.toBeInTheDocument()
  })

  it("shows profile module for profile pages and hides removed modules", async () => {
    useAuthStore.setState({
      user: {
        id: "user-2",
        username: "bob",
        email: "bob@example.com",
        is_admin: false,
        permissions: [],
        email_verified: true,
      },
      initStatus: "ready",
    })

    renderShell("/profile")

    expect(await screen.findByRole("combobox", { name: "Subsystem" })).toHaveTextContent("Profile")
    expect(screen.getByRole("link", { name: "Profile" })).toHaveAttribute("aria-current", "page")
    expect(screen.getByRole("link", { name: "Points Center" })).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Tools" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Quotes" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Monitoring" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument()
  })

  it("keeps module switching available after sidebar collapse", async () => {
    useAuthStore.setState({
      user: {
        id: "user-4",
        username: "dave",
        email: "dave@example.com",
        is_admin: false,
        permissions: [],
        email_verified: true,
      },
      initStatus: "ready",
    })

    renderShell("/profile")

    const user = userEvent.setup()
    await screen.findByRole("button", { name: "collapse sidebar" })
    await user.click(screen.getByRole("button", { name: "collapse sidebar" }))
    await user.click(screen.getByRole("button", { name: "Subsystem" }))
    await user.click(screen.getByRole("menuitem", { name: "Home" }))

    expect(screen.getByTestId("pathname")).toHaveTextContent("/")
  })

  it("shows admin tools group inside admin module", async () => {
    useAuthStore.setState({
      user: {
        id: "user-3",
        username: "carol",
        email: "carol@example.com",
        is_admin: false,
        permissions: [adminPermissions.usersRead, adminPermissions.pointsRead, adminPermissions.settingsRead],
        email_verified: true,
      },
      initStatus: "ready",
    })

    renderShell("/admin/tools/points")

    expect(await screen.findByRole("combobox", { name: "Subsystem" })).toHaveTextContent("Admin")
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument()
    expect(screen.getAllByText("Tools").length).toBeGreaterThan(0)
    expect(screen.getByRole("link", { name: "Points Management" })).toHaveAttribute("aria-current", "page")
    expect(screen.queryByRole("link", { name: "Quote Moderation" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Applications" })).not.toBeInTheDocument()
  })
})
