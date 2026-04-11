import { MemoryRouter } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import AppsPage from "@/pages/apps"

const listMock = vi.fn()
const statsMock = vi.fn()

vi.mock("@/api/services", () => ({
  userAppApi: {
    list: (...args: unknown[]) => listMock(...args),
  },
  statsApi: {
    user: (...args: unknown[]) => statsMock(...args),
  },
}))

describe("AppsPage", () => {
  beforeEach(() => {
    listMock.mockReset()
    statsMock.mockReset()
  })

  it("hides workspace column in the template app list", async () => {
    listMock.mockResolvedValue({
      data: {
        applications: [
          {
            id: "app-1",
            name: "Acme Docs",
            client_id: "client-1",
            scopes: "openid profile email",
          },
        ],
        total: 1,
      },
    })
    statsMock.mockResolvedValue({
      data: {
        app_stats: [],
      },
    })

    render(
      <MemoryRouter>
        <AppsPage />
      </MemoryRouter>
    )

    expect(await screen.findByText("Acme Docs")).toBeInTheDocument()
    expect(screen.queryByRole("columnheader", { name: "Workspace" })).not.toBeInTheDocument()
    expect(screen.queryByText("Personal workspace")).not.toBeInTheDocument()
  })
})
