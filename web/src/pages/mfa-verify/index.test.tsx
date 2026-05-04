import { MemoryRouter } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import MFAVerifyPage from "@/pages/mfa-verify"

const mfaVerifyMock = vi.fn()

vi.mock("@/api/services", () => ({
  authApi: {
    mfaVerify: (...args: unknown[]) => mfaVerifyMock(...args),
  },
}))

describe("MFAVerifyPage", () => {
  beforeEach(() => {
    mfaVerifyMock.mockReset()
  })

  it("submits mfa code for browser login", async () => {
    mfaVerifyMock.mockResolvedValue({
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
      <MemoryRouter initialEntries={["/mfa-verify?mfa_token=token-1"]}>
        <MFAVerifyPage />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(screen.getByLabelText("Verification Code"), "123456")
    await user.click(screen.getByRole("button", { name: "Verify" }))

    expect(mfaVerifyMock).toHaveBeenCalledWith("token-1", "123456", undefined)
  })
})
