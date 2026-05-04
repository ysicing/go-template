import { MemoryRouter } from "react-router-dom"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import OAuthClientEditor from "@/components/oauth-client/OAuthClientEditor"

const createMock = vi.fn()

describe("OAuthClientEditor", () => {
  beforeEach(() => {
    createMock.mockReset()
  })

  it("submits machine client form data", async () => {
    createMock.mockResolvedValue({
      client_secret: "secret-1",
      client: { client_id: "client-1" },
    })

    render(
      <MemoryRouter>
        <OAuthClientEditor
          namespace="clients"
          backPath="/admin/clients"
          onCreate={createMock}
          onGet={vi.fn()}
          onUpdate={vi.fn()}
          showCreatedClientId
        />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(screen.getByLabelText("Name"), "Acme Worker")
    await user.clear(screen.getByLabelText("Scopes"))
    await user.type(screen.getByLabelText("Scopes"), "read,write")
    await user.click(screen.getByRole("button", { name: "Save" }))

    expect(createMock).toHaveBeenCalledWith({
      name: "Acme Worker",
      scopes: "read,write",
    })
  })

  it("shows machine endpoint guidance", () => {
    render(
      <MemoryRouter>
        <OAuthClientEditor
          namespace="clients"
          backPath="/admin/clients"
          onCreate={createMock}
          onGet={vi.fn()}
          onUpdate={vi.fn()}
        />
      </MemoryRouter>
    )

    expect(screen.getByText(/machine token scopes/i)).toBeInTheDocument()
  })
})
