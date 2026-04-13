import { MemoryRouter } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import AdminAuditLogsPage from "@/pages/admin-audit-logs"
import { adminPermissions } from "@/lib/permissions"
import { useAuthStore } from "@/stores/auth"

const auditLogListMock = vi.fn()

vi.mock("@/api/services", () => ({
  auditLogApi: {
    list: (...args: unknown[]) => auditLogListMock(...args),
  },
}))

describe("AdminAuditLogsPage", () => {
  beforeEach(() => {
    auditLogListMock.mockReset()
    useAuthStore.setState({
      user: {
        id: "admin-1",
        username: "admin",
        email: "admin@example.com",
        is_admin: false,
        permissions: [adminPermissions.loginHistoryRead],
        email_verified: true,
      },
      initStatus: "ready",
    })
  })

  it("opens a dialog to show full audit detail content", async () => {
    auditLogListMock.mockResolvedValue({
      data: {
        logs: [
          {
            id: "log-1",
            user_id: "user-1",
            username: "alice",
            action: "login",
            resource: "user",
            resource_id: "user-1",
            client_id: "",
            ip: "127.0.0.1",
            user_agent: "Mozilla/5.0 test browser",
            detail: "line 1\\nline 2 with a much longer explanation for audit trace",
            status: "success",
            source: "web",
            created_at: "2026-04-12T10:00:00Z",
          },
        ],
        total: 1,
      },
    })

    render(
      <MemoryRouter>
        <AdminAuditLogsPage />
      </MemoryRouter>,
    )

    const user = userEvent.setup()
    await user.click(await screen.findByRole("button", { name: "View detail" }))

    const dialog = screen.getByRole("dialog")
    expect(dialog).toBeInTheDocument()
    expect(dialog).toHaveTextContent("line 1")
    expect(dialog).toHaveTextContent("line 2 with a much longer explanation for audit trace")
  })
})
