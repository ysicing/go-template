import { MemoryRouter } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import MFAVerifyPage from "@/pages/mfa-verify"

const mfaVerifyMock = vi.fn()
const redirectToSameOriginMock = vi.fn()

vi.mock("@/api/services", () => ({
  authApi: {
    mfaVerify: (...args: unknown[]) => mfaVerifyMock(...args),
  },
}))

vi.mock("@/lib/navigation", () => ({
  redirectToSameOrigin: (...args: unknown[]) => redirectToSameOriginMock(...args),
}))

describe("MFAVerifyPage", () => {
  beforeEach(() => {
    mfaVerifyMock.mockReset()
    redirectToSameOriginMock.mockReset()
  })

  it("redirects oidc consent flows to consent page", async () => {
    mfaVerifyMock.mockResolvedValue({
      data: {
        redirect: "/consent?id=req-1",
      },
    })
    redirectToSameOriginMock.mockReturnValue(true)

    render(
      <MemoryRouter initialEntries={["/mfa-verify?mfa_token=token-1"]}>
        <MFAVerifyPage />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(screen.getByLabelText("Verification Code"), "123456")
    await user.click(screen.getByRole("button", { name: "Verify" }))

    expect(mfaVerifyMock).toHaveBeenCalledWith("token-1", "123456", undefined)
    await waitFor(() => {
      expect(redirectToSameOriginMock).toHaveBeenCalledWith("/consent?id=req-1")
    })
  })
})
