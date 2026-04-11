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

  it("submits require_consent for application forms", async () => {
    createMock.mockResolvedValue({
      client_secret: "secret-1",
      client: { client_id: "client-1" },
    })

    render(
      <MemoryRouter>
        <OAuthClientEditor
          namespace="apps"
          backPath="/uauth/apps"
          onCreate={createMock}
          onGet={vi.fn()}
          onUpdate={vi.fn()}
          showCreatedClientId
        />
      </MemoryRouter>
    )

    const user = userEvent.setup()
    await user.type(screen.getByLabelText("Name"), "Acme Docs")
    await user.type(screen.getByLabelText("Redirect URIs"), "https://example.com/callback")
    await user.click(screen.getByRole("checkbox", { name: "Require consent" }))
    await user.click(screen.getByRole("button", { name: "Save" }))

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: "Acme Docs",
      redirect_uris: "https://example.com/callback",
      require_consent: true,
    }))
  })

  it("shows supported grant type guidance including client_credentials", () => {
    render(
      <MemoryRouter>
        <OAuthClientEditor
          namespace="apps"
          backPath="/uauth/apps"
          onCreate={createMock}
          onGet={vi.fn()}
          onUpdate={vi.fn()}
        />
      </MemoryRouter>
    )

    expect(screen.getByText(/client_credentials/i)).toBeInTheDocument()
  })

  it("does not show workspace selection for template application forms", () => {
    render(
      <MemoryRouter>
        <OAuthClientEditor
          namespace="apps"
          backPath="/uauth/apps"
          onCreate={createMock}
          onGet={vi.fn()}
          onUpdate={vi.fn()}
        />
      </MemoryRouter>
    )

    expect(screen.queryByLabelText("Workspace")).not.toBeInTheDocument()
  })
})
