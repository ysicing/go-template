import { MemoryRouter, useLocation } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import RegisterPage from "@/pages/register"

const authConfigMock = vi.fn()
const registerMock = vi.fn()

vi.mock("@/api/services", () => ({
  authApi: {
    config: (...args: unknown[]) => authConfigMock(...args),
    register: (...args: unknown[]) => registerMock(...args),
  },
}))

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}{location.search}</div>
}

describe("RegisterPage", () => {
  beforeEach(() => {
    authConfigMock.mockReset()
    registerMock.mockReset()
    authConfigMock.mockResolvedValue({
      data: {
        turnstile_site_key: "",
      },
    })
  })

  it("redirects signups to home when email verification is not required", async () => {
    registerMock.mockResolvedValue({
      data: {
        user: {
          id: "user-1",
          username: "alice",
          email: "alice@example.com",
          is_admin: false,
          permissions: [],
          email_verified: true,
        },
        email_verification_required: false,
      },
    })

    render(
      <MemoryRouter initialEntries={["/register"]}>
        <RegisterPage />
        <LocationProbe />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(await screen.findByLabelText("Username"), "alice")
    await user.type(screen.getByLabelText("Email"), "alice@example.com")
    await user.type(screen.getByLabelText("Password"), "Password123!abcd")
    await user.click(screen.getByRole("button", { name: "Sign Up" }))

    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/")
    })
  })
})
