import { MemoryRouter, Route, Routes } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import AppViewPage from "@/pages/apps/view"

const getAppMock = vi.fn()
const rotateSecretMock = vi.fn()
const userStatsMock = vi.fn()

vi.mock("@/api/services", () => ({
  userAppApi: {
    get: (...args: unknown[]) => getAppMock(...args),
    rotateSecret: (...args: unknown[]) => rotateSecretMock(...args),
  },
  statsApi: {
    user: (...args: unknown[]) => userStatsMock(...args),
  },
}))

describe("AppViewPage", () => {
  beforeEach(() => {
    getAppMock.mockReset()
    rotateSecretMock.mockReset()
    userStatsMock.mockReset()
  })

  it("renders template application details without machine access controls", async () => {
    getAppMock.mockResolvedValue({
      data: {
        application: {
          name: "Acme Docs",
          client_id: "client-1",
          redirect_uris: "https://example.com/callback",
          grant_types: "authorization_code client_credentials",
          scopes: "openid profile email",
          require_consent: true,
        },
      },
    })
    userStatsMock.mockResolvedValue({
      data: {
        app_stats: [
          {
            client_id: "client-1",
            user_count: 5,
            login_count: 12,
          },
        ],
      },
    })

    render(
      <MemoryRouter initialEntries={["/uauth/apps/app-1/view"]}>
        <Routes>
          <Route path="/uauth/apps/:id/view" element={<AppViewPage />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByRole("heading", { name: "Acme Docs" })).toBeInTheDocument()
    expect(screen.getByText("Consent policy")).toBeInTheDocument()
    expect(screen.getByText("Require consent")).toBeInTheDocument()
    expect(screen.getByText("Enabled")).toBeInTheDocument()
    expect(screen.getByText("Users")).toBeInTheDocument()
    expect(screen.getByText("Logins")).toBeInTheDocument()
    expect(screen.getByText("5")).toBeInTheDocument()
    expect(screen.getByText("12")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Rotate secret" })).toBeInTheDocument()
    expect(screen.queryByText("Machine access")).not.toBeInTheDocument()
    expect(screen.queryByText("Token lifecycle")).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Introspect token" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Revoke token" })).not.toBeInTheDocument()
  })

  it("reveals rotated secret for template applications", async () => {
    getAppMock.mockResolvedValue({
      data: {
        application: {
          id: "app-1",
          name: "Acme Docs",
          client_id: "client-1",
          redirect_uris: "https://example.com/callback",
          grant_types: "authorization_code",
          scopes: "openid profile email",
          require_consent: true,
        },
      },
    })
    userStatsMock.mockResolvedValue({
      data: {
        app_stats: [],
      },
    })
    rotateSecretMock.mockResolvedValue({
      data: {
        client_secret: "rotated-secret-123",
      },
    })

    render(
      <MemoryRouter initialEntries={["/uauth/apps/app-1/view"]}>
        <Routes>
          <Route path="/uauth/apps/:id/view" element={<AppViewPage />} />
        </Routes>
      </MemoryRouter>
    )

    const user = userEvent.setup()

    expect(await screen.findByRole("heading", { name: "Acme Docs" })).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Rotate secret" }))

    expect(rotateSecretMock).toHaveBeenCalledWith("app-1")
    expect(await screen.findByText("rotated-secret-123")).toBeInTheDocument()
    expect(screen.getByText("Shown once after create or rotate. Copy it now.")).toBeInTheDocument()
  })
})
