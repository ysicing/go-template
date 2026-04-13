import { MemoryRouter, useLocation } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import LoginPage from "@/pages/login"

const authConfigMock = vi.fn()
const loginMock = vi.fn()
const oidcLoginMock = vi.fn()
const confirmSocialLinkMock = vi.fn()
const versionGetMock = vi.fn()
const redirectToSameOriginMock = vi.fn()

vi.mock("@/api/services", () => ({
  authApi: {
    config: (...args: unknown[]) => authConfigMock(...args),
    login: (...args: unknown[]) => loginMock(...args),
    oidcLogin: (...args: unknown[]) => oidcLoginMock(...args),
    confirmSocialLink: (...args: unknown[]) => confirmSocialLinkMock(...args),
  },
  versionApi: {
    get: (...args: unknown[]) => versionGetMock(...args),
  },
}))

vi.mock("@/lib/navigation", () => ({
  redirectToSameOrigin: (...args: unknown[]) => redirectToSameOriginMock(...args),
}))

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}{location.search}</div>
}

describe("LoginPage", () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { ResizeObserver?: typeof ResizeObserver }).ResizeObserver = class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    } as unknown as typeof ResizeObserver
    authConfigMock.mockReset()
    loginMock.mockReset()
    oidcLoginMock.mockReset()
    confirmSocialLinkMock.mockReset()
    versionGetMock.mockReset()
    redirectToSameOriginMock.mockReset()
    versionGetMock.mockResolvedValue({ data: { git_commit: "", build_date: "" } })
    authConfigMock.mockResolvedValue({
      data: {
        register_enabled: true,
        turnstile_site_key: "",
      },
    })
  })

  it("renders oidc branding when auth config includes branding", async () => {
    authConfigMock.mockResolvedValue({
      data: {
        register_enabled: true,
        turnstile_site_key: "",
        branding: {
          display_name: "Acme Cloud",
          headline: "Secure access for Acme employees",
          logo_url: "https://cdn.example.com/acme.png",
          primary_color: "#2563eb",
        },
      },
    })
    versionGetMock.mockResolvedValue({ data: { version: "v1.0.0", git_commit: "abc1234def5678", build_date: "2026-04-12T10:00:00Z" } })

    render(
      <MemoryRouter initialEntries={["/login?id=req-1"]}>
        <LoginPage />
      </MemoryRouter>
    )

    expect(await screen.findByText("Acme Cloud")).toBeInTheDocument()
    expect(screen.getByText("Secure access for Acme employees")).toBeInTheDocument()
    expect(screen.getByText("v1.0.0 · abc1234")).toBeInTheDocument()
  })

  it("redirects oidc consent flows to consent page", async () => {
    authConfigMock.mockResolvedValue({
      data: {
        register_enabled: true,
        turnstile_site_key: "",
      },
    })
    oidcLoginMock.mockResolvedValue({
      data: {
        redirect: "/consent?id=req-1",
      },
    })
    redirectToSameOriginMock.mockReturnValue(true)

    render(
      <MemoryRouter initialEntries={["/login?id=req-1"]}>
        <LoginPage />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(await screen.findByLabelText("Username / Email"), "alice")
    await user.type(screen.getByLabelText("Password"), "Password123!abcd")
    await user.click(screen.getByRole("button", { name: "Sign In" }))

    expect(oidcLoginMock).toHaveBeenCalledWith("req-1", "alice", "Password123!abcd")
    await waitFor(() => {
      expect(redirectToSameOriginMock).toHaveBeenCalledWith("/consent?id=req-1")
    })
  })

  it("redirects normal logins to home", async () => {
    loginMock.mockResolvedValue({
      data: {
        user: {
          id: "user-1",
          username: "alice",
          email: "alice@example.com",
          is_admin: false,
          permissions: [],
          email_verified: true,
        },
      },
    })

    render(
      <MemoryRouter initialEntries={["/login"]}>
        <LoginPage />
        <LocationProbe />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(await screen.findByLabelText("Username / Email"), "alice")
    await user.type(screen.getByLabelText("Password"), "Password123!abcd")
    await user.click(screen.getByRole("button", { name: "Sign In" }))

    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/")
    })
  })

  it("shows confirm-link mode when social link params exist", async () => {
    render(
      <MemoryRouter initialEntries={["/login?link_required=true&link_token=link-123&provider=github"]}>
        <LoginPage />
      </MemoryRouter>
    )

    expect(await screen.findByText(/confirm github link/i)).toBeInTheDocument()
    expect(screen.getByText(/we detected an existing account for this email/i)).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: "Password" })).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: "TOTP" })).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: "Passkey" })).toBeInTheDocument()
  })

  it("submits password confirmation for social link", async () => {
    confirmSocialLinkMock.mockResolvedValue({
      data: {
        user: {
          id: "user-1",
          username: "alice",
          email: "alice@example.com",
          is_admin: false,
          permissions: [],
          email_verified: true,
        },
      },
    })

    render(
      <MemoryRouter initialEntries={["/login?link_required=true&link_token=link-123&provider=github"]}>
        <LoginPage />
        <LocationProbe />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(await screen.findByLabelText("Confirm with password"), "Password123!abcd")
    await user.click(screen.getByRole("button", { name: "Confirm and link GitHub" }))

    expect(confirmSocialLinkMock).toHaveBeenCalledWith("link-123", { password: "Password123!abcd" })
    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/")
    })
  })
})
