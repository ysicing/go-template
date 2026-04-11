import { MemoryRouter } from "react-router-dom"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import ConsentPage from "@/pages/consent"

const consentContextMock = vi.fn()
const consentApproveMock = vi.fn()
const consentDenyMock = vi.fn()
const redirectToSameOriginMock = vi.fn()

vi.mock("@/api/services", () => ({
  authApi: {
    oidcConsentContext: (...args: unknown[]) => consentContextMock(...args),
    oidcConsentApprove: (...args: unknown[]) => consentApproveMock(...args),
    oidcConsentDeny: (...args: unknown[]) => consentDenyMock(...args),
  },
}))

vi.mock("@/lib/navigation", () => ({
  redirectToSameOrigin: (...args: unknown[]) => redirectToSameOriginMock(...args),
}))

describe("ConsentPage", () => {
  beforeEach(() => {
    consentContextMock.mockReset()
    consentApproveMock.mockReset()
    consentDenyMock.mockReset()
    redirectToSameOriginMock.mockReset()
  })

  it("renders loading state before bootstrap resolves", () => {
    consentContextMock.mockReturnValue(new Promise(() => {}))

    const { container } = render(
      <MemoryRouter initialEntries={["/consent?id=req-1"]}>
        <ConsentPage />
      </MemoryRouter>
    )

    expect(container.querySelector(".animate-spin")).toBeTruthy()
  })

  it("renders page error state when bootstrap fails", async () => {
    consentContextMock.mockRejectedValue(new Error("boom"))

    render(
      <MemoryRouter initialEntries={["/consent?id=req-1"]}>
        <ConsentPage />
      </MemoryRouter>
    )

    expect(await screen.findByRole("button", { name: /retry/i })).toBeInTheDocument()
  })

  it("renders branded consent details and approves the request", async () => {
    consentContextMock.mockResolvedValue({
      data: {
        client: { id: "client-1", name: "Acme Docs" },
        scopes: ["openid", "profile", "email"],
        branding: {
          display_name: "Acme Cloud",
          headline: "Secure access for Acme employees",
          logo_url: "https://cdn.example.com/acme.png",
          primary_color: "#2563eb",
        },
        requires_consent: true,
      },
    })
    consentApproveMock.mockResolvedValue({
      data: {
        redirect: "/authorize/callback?id=req-1",
      },
    })
    redirectToSameOriginMock.mockReturnValue(true)

    render(
      <MemoryRouter initialEntries={["/consent?id=req-1"]}>
        <ConsentPage />
      </MemoryRouter>
    )

    expect(await screen.findByText("Acme Cloud")).toBeInTheDocument()
    expect(screen.getByText("Acme Docs")).toBeInTheDocument()
    expect(screen.getByText("openid")).toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(screen.getByRole("button", { name: "Allow" }))

    expect(consentApproveMock).toHaveBeenCalledWith("req-1")
    await waitFor(() => {
      expect(redirectToSameOriginMock).toHaveBeenCalledWith("/authorize/callback?id=req-1")
    })
  })
})
