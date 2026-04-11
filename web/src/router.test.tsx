import { MemoryRouter, Outlet } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { useAuthStore } from "@/stores/auth"

vi.mock("@/layouts/AppShell", () => ({
  default: () => <Outlet />,
}))

vi.mock("@/pages/dashboard", () => ({
  default: () => <div>Dashboard Screen</div>,
}))

import AppRouter from "@/router"

describe("AppRouter", () => {
  it("redirects removed monitoring routes back to dashboard", async () => {
    useAuthStore.setState({
      user: {
        id: "user-1",
        username: "alice",
        email: "alice@example.com",
        is_admin: true,
        permissions: [],
        email_verified: true,
      },
      initStatus: "ready",
    })

    render(
      <MemoryRouter initialEntries={["/monitoring"]}>
        <AppRouter />
      </MemoryRouter>
    )

    expect(await screen.findByText("Dashboard Screen")).toBeInTheDocument()
  })
})
