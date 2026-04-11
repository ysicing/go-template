import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { AuthorizedAppsTab } from "@/pages/profile/authorized-apps-tab"

const listAuthorizedAppsMock = vi.fn()
const revokeAuthorizedAppMock = vi.fn()

vi.mock("@/api/services", () => ({
  userApi: {
    listAuthorizedApps: (...args: unknown[]) => listAuthorizedAppsMock(...args),
    revokeAuthorizedApp: (...args: unknown[]) => revokeAuthorizedAppMock(...args),
  },
}))

describe("AuthorizedAppsTab", () => {
  beforeEach(() => {
    listAuthorizedAppsMock.mockReset()
    revokeAuthorizedAppMock.mockReset()
  })

  it("renders authorized apps and revokes one entry", async () => {
    listAuthorizedAppsMock
      .mockResolvedValueOnce({
        data: {
          apps: [
            {
              id: "grant-1",
              client_name: "Acme Docs",
              client_id: "client-1",
              scopes: "openid profile",
              granted_at: "2026-04-04T00:00:00Z",
            },
          ],
          total: 1,
          page: 1,
          page_size: 10,
        },
      })
      .mockResolvedValueOnce({
        data: {
          apps: [],
          total: 0,
          page: 1,
          page_size: 10,
        },
      })
    revokeAuthorizedAppMock.mockResolvedValue({ data: { message: "authorized app revoked" } })

    render(<AuthorizedAppsTab />)

    expect(await screen.findByText("Acme Docs")).toBeInTheDocument()
    expect(screen.getByText("client-1")).toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(screen.getByRole("button", { name: "Revoke" }))

    expect(revokeAuthorizedAppMock).toHaveBeenCalledWith("grant-1")
    await waitFor(() => {
      expect(screen.getByText("No authorized apps yet")).toBeInTheDocument()
    })
  })

  it("renders page error state on bootstrap failure", async () => {
    listAuthorizedAppsMock.mockRejectedValue(new Error("boom"))

    render(<AuthorizedAppsTab />)

    expect(await screen.findByRole("button", { name: /retry/i })).toBeInTheDocument()
  })
})
