import { useCallback, useEffect, useRef, useState } from "react"
import { useNavigate, useSearchParams, Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { AlertCircle, Fingerprint, KeyRound, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { BuildVersion } from "@/components/BuildVersion"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { authApi, versionApi } from "@/api/services"
import { getErrorMessage } from "@/api/client"
import { getBuildVersionLabel } from "@/lib/build-version"
import { useAuthStore, type User } from "@/stores/auth"
import { useAppStore } from "@/stores/app"
import { redirectToSameOrigin } from "@/lib/navigation"

declare global {
  interface Window {
    turnstile?: {
      render: (container: string | HTMLElement, options: Record<string, unknown>) => string
      reset: (widgetId: string) => void
      remove: (widgetId: string) => void
    }
  }
}

export default function LoginPage() {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [webauthnLoading, setWebauthnLoading] = useState(false)
  const [linkLoading, setLinkLoading] = useState(false)
  const [registerEnabled, setRegisterEnabled] = useState(false)
  const [turnstileSiteKey, setTurnstileSiteKey] = useState("")
  const [turnstileToken, setTurnstileToken] = useState("")
  const [rememberMe, setRememberMe] = useState(true)
  const [linkRequired, setLinkRequired] = useState(false)
  const [linkToken, setLinkToken] = useState("")
  const [linkProvider, setLinkProvider] = useState("")
  const [linkMethod, setLinkMethod] = useState<"password" | "totp" | "webauthn">("password")
  const [linkPassword, setLinkPassword] = useState("")
  const [linkTotpCode, setLinkTotpCode] = useState("")
  const [versionInfo, setVersionInfo] = useState({ version: "", git_commit: "", build_date: "" })
  const [branding, setBranding] = useState<null | {
    display_name?: string
    headline?: string
    logo_url?: string
    primary_color?: string
  }>(null)
  const turnstileRef = useRef<HTMLDivElement>(null)
  const widgetIdRef = useRef<string | null>(null)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()
  const { setUser } = useAuthStore()
  const { themeMode } = useAppStore()

  // OIDC auth request id (present when redirected from OIDC flow)
  const oidcId = searchParams.get("id")
  const linkProviderLabel = formatProviderName(linkProvider)

  // Show error from social login redirect (e.g. email_required, account_locked)
  useEffect(() => {
    const err = searchParams.get("error")
    if (err === "email_required") {
      toast.error(t("login.emailRequired"))
    } else if (err === "account_locked") {
      toast.error(t("login.accountLocked"))
    }
  }, [searchParams, t])

  useEffect(() => {
    const required = searchParams.get("link_required") === "true"
    const token = searchParams.get("link_token") || ""
    const provider = searchParams.get("provider") || ""

    if (required && token) {
      setLinkRequired(true)
      setLinkToken(token)
      setLinkProvider(provider)
      setLinkMethod("password")
      return
    }

    setLinkRequired(false)
    setLinkToken("")
    setLinkProvider("")
    setLinkPassword("")
    setLinkTotpCode("")
  }, [searchParams])

  useEffect(() => {
    authApi.config(oidcId || undefined).then((res) => {
      setRegisterEnabled(res.data.register_enabled)
      setTurnstileSiteKey(res.data.turnstile_site_key || "")
      setBranding(res.data.branding || null)
    }).catch(() => {})
    versionApi.get().then((res) => setVersionInfo(res.data)).catch(() => {})
  }, [oidcId])

  const brandStyle = branding?.primary_color ? { backgroundColor: branding.primary_color, borderColor: branding.primary_color } : undefined
  const versionLabel = getBuildVersionLabel(versionInfo)

  const renderTurnstile = useCallback(() => {
    if (!turnstileSiteKey || !turnstileRef.current || !window.turnstile) return
    if (widgetIdRef.current) {
      window.turnstile.remove(widgetIdRef.current)
    }
    widgetIdRef.current = window.turnstile.render(turnstileRef.current, {
      sitekey: turnstileSiteKey,
      theme: themeMode === "dark" ? "dark" : "light",
      callback: (token: string) => setTurnstileToken(token),
      "expired-callback": () => setTurnstileToken(""),
    })
  }, [turnstileSiteKey, themeMode])

  useEffect(() => {
    if (!turnstileSiteKey) return
    if (window.turnstile) {
      renderTurnstile()
      return
    }
    const script = document.createElement("script")
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
    script.async = true
    script.onload = () => renderTurnstile()
    document.head.appendChild(script)
    return () => {
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = null
      }
    }
  }, [turnstileSiteKey, renderTurnstile])

  const resetTurnstile = () => {
    if (widgetIdRef.current && window.turnstile) {
      window.turnstile.reset(widgetIdRef.current)
      setTurnstileToken("")
    }
  }

  const handleLogin = (data: { user: User }) => {
    // Tokens are automatically set in HttpOnly cookies by the server
    setUser(data.user)
    navigate("/")
  }

  const handleAuthResult = (data: { user?: User; mfa_required?: boolean; mfa_token?: string }) => {
    if (data.mfa_required && data.mfa_token) {
      navigate(`/mfa-verify?mfa_token=${encodeURIComponent(data.mfa_token)}`)
      return
    }
    if (data.user) {
      handleLogin({ user: data.user })
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      if (oidcId) {
        const res = await authApi.oidcLogin(oidcId, username, password)
        if (res.data.mfa_required) {
          navigate(`/mfa-verify?mfa_token=${encodeURIComponent(res.data.mfa_token)}`)
          return
        }
        // Validate redirect is same-origin to prevent open redirect attacks
        const redirect = res.data.redirect
        if (redirect) {
          if (!redirectToSameOrigin(redirect)) {
            navigate("/")
          }
        } else {
          navigate("/")
        }
        return
      }
      const res = await authApi.login(username, password, turnstileToken || undefined, rememberMe)
      handleAuthResult(res.data)
    } catch (error) {
      toast.error(getErrorMessage(error, t("login.error")))
    } finally {
      setLoading(false)
      resetTurnstile()
    }
  }

  const handleWebAuthn = async () => {
    if (!username) {
      toast.error(t("login.usernameRequired"))
      return
    }
    setWebauthnLoading(true)
    try {
      const beginRes = await authApi.webauthnLoginBegin(username)
      const { publicKey, webauthn_token } = beginRes.data

      const body = await getWebAuthnAssertionBody(publicKey)
      const finishRes = await authApi.webauthnLoginFinish(webauthn_token, body)
      handleAuthResult(finishRes.data)
    } catch (error) {
      toast.error(getErrorMessage(error, t("login.webauthnFailed")))
    } finally {
      setWebauthnLoading(false)
    }
  }

  const exitLinkMode = () => {
    setLinkRequired(false)
    setLinkToken("")
    setLinkProvider("")
    setLinkMethod("password")
    setLinkPassword("")
    setLinkTotpCode("")
    navigate("/login", { replace: true })
  }

  const handleLinkConfirm = async () => {
    if (!linkToken) {
      toast.error(t("login.linkExpired"))
      return
    }

    setLinkLoading(true)
    try {
      if (linkMethod === "password") {
        const res = await authApi.confirmSocialLink(linkToken, { password: linkPassword })
        handleAuthResult(res.data)
        return
      }

      if (linkMethod === "totp") {
        const res = await authApi.confirmSocialLink(linkToken, { totp_code: linkTotpCode })
        handleAuthResult(res.data)
        return
      }

      if (!navigator.credentials || !window.PublicKeyCredential) {
        toast.error(t("login.passkeyUnavailable"))
        return
      }

      const beginRes = await authApi.socialLinkWebAuthnBegin(linkToken)
      const body = await getWebAuthnAssertionBody(beginRes.data.publicKey)
      const finishRes = await authApi.socialLinkWebAuthnFinish(linkToken, body)
      handleAuthResult(finishRes.data)
    } catch (error) {
      toast.error(getErrorMessage(error, t("login.linkConfirmFailed")))
    } finally {
      setLinkLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <div className="flex flex-1 items-center justify-center p-4">
        <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground overflow-hidden" style={brandStyle}>
            {branding?.logo_url ? (
              <img src={branding.logo_url} alt={branding.display_name || t("app.title")} className="h-full w-full object-cover" />
            ) : (
              <KeyRound className="h-6 w-6" />
            )}
          </div>
          <h1 className="text-xl font-semibold">{branding?.display_name || t("app.title")}</h1>
          <p className="text-sm text-muted-foreground">
            {linkRequired ? t("login.linkTitle", { provider: linkProviderLabel }) : (branding?.headline || t("login.title"))}
          </p>
        </CardHeader>
        <CardContent>
          {linkRequired ? (
            <>
              <Alert className="mb-4">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>{t("login.linkAlertTitle")}</AlertTitle>
                <AlertDescription>{t("login.linkAlertDescription")}</AlertDescription>
              </Alert>
              <Tabs value={linkMethod} onValueChange={(value) => setLinkMethod(value as "password" | "totp" | "webauthn")} className="space-y-4">
                <TabsList className="grid w-full grid-cols-3">
                  <TabsTrigger value="password">{t("login.linkMethodPassword")}</TabsTrigger>
                  <TabsTrigger value="totp">{t("login.linkMethodTotp")}</TabsTrigger>
                  <TabsTrigger value="webauthn">{t("login.linkMethodPasskey")}</TabsTrigger>
                </TabsList>
                <TabsContent value="password" className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="link-password">{t("login.linkPasswordLabel")}</Label>
                    <Input
                      id="link-password"
                      type="password"
                      value={linkPassword}
                      onChange={(e) => setLinkPassword(e.target.value)}
                      autoFocus
                    />
                  </div>
                </TabsContent>
                <TabsContent value="totp" className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="link-totp">{t("login.linkTotpLabel")}</Label>
                    <Input
                      id="link-totp"
                      value={linkTotpCode}
                      onChange={(e) => setLinkTotpCode(e.target.value)}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      maxLength={6}
                    />
                  </div>
                </TabsContent>
                <TabsContent value="webauthn" className="space-y-4">
                  <p className="text-sm text-muted-foreground">{t("login.linkPasskeyDescription")}</p>
                </TabsContent>
              </Tabs>
              <div className="space-y-3">
                <Button
                  type="button"
                  className="w-full"
                  style={brandStyle}
                  onClick={handleLinkConfirm}
                  disabled={linkLoading || (linkMethod === "password" && !linkPassword) || (linkMethod === "totp" && !linkTotpCode)}
                >
                  {linkLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : (linkMethod === "webauthn" ? <Fingerprint className="mr-2 h-4 w-4" /> : null)}
                  {t("login.linkConfirmButton", { provider: linkProviderLabel })}
                </Button>
                <Button type="button" variant="ghost" className="w-full" onClick={exitLinkMode}>
                  {t("login.backToLogin")}
                </Button>
              </div>
            </>
          ) : (
            <>
              <form onSubmit={handleSubmit} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="username">{t("login.username")}</Label>
                  <Input
                    id="username"
                    placeholder={t("login.username")}
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    required
                    autoFocus
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="password">{t("login.password")}</Label>
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="rememberMe"
                    checked={rememberMe}
                    onCheckedChange={(checked) => setRememberMe(checked === true)}
                  />
                  <Label htmlFor="rememberMe" className="text-sm font-normal cursor-pointer">
                    {t("login.rememberMe")}
                  </Label>
                </div>
                {turnstileSiteKey && (
                  <div ref={turnstileRef} className="flex justify-center" />
                )}
                <Button type="submit" className="w-full" style={brandStyle} disabled={loading || (!!turnstileSiteKey && !turnstileToken)}>
                  {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {t("login.submit")}
                </Button>
              </form>

              {!oidcId && (
                <>
                  <div className="relative my-4">
                    <Separator />
                    <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-card px-2 text-xs text-muted-foreground">
                      {t("login.or")}
                    </span>
                  </div>
                  <Button
                    variant="outline"
                    className="w-full"
                    onClick={handleWebAuthn}
                    disabled={webauthnLoading}
                  >
                    {webauthnLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Fingerprint className="mr-2 h-4 w-4" />}
                    {t("login.webauthn")}
                  </Button>
                </>
              )}

              {registerEnabled && !oidcId && (
                <p className="mt-4 text-center text-sm text-muted-foreground">
                  {t("login.noAccount")}{" "}
                  <Link to="/register" className="text-primary hover:underline">
                    {t("login.register")}
                  </Link>
                </p>
              )}
            </>
          )}
        </CardContent>
      </Card>
      </div>
      {/* Footer with version info */}
      {versionLabel && (
        <footer className="py-4 text-center text-xs text-muted-foreground">
          <BuildVersion info={versionInfo} className="block" />
        </footer>
      )}
    </div>
  )
}

async function getWebAuthnAssertionBody(publicKey: {
  challenge: string
  allowCredentials?: Array<{ id: string } & Record<string, unknown>>
} & Record<string, unknown>) {
  const normalizedPublicKey = normalizeAssertionOptions(publicKey)
  const credential = await navigator.credentials.get({ publicKey: normalizedPublicKey }) as PublicKeyCredential
  return credentialToAssertionBody(credential)
}

function normalizeAssertionOptions(publicKey: {
  challenge: string
  allowCredentials?: Array<{ id: string; type?: PublicKeyCredentialType; transports?: AuthenticatorTransport[] } & Record<string, unknown>>
} & Record<string, unknown>): PublicKeyCredentialRequestOptions {
  const normalized: PublicKeyCredentialRequestOptions = {
    ...publicKey,
    challenge: base64urlToBuffer(publicKey.challenge),
    allowCredentials: publicKey.allowCredentials?.map((cred) => ({
      ...cred,
      id: base64urlToBuffer(cred.id),
      type: cred.type ?? "public-key",
    })),
  }
  return normalized
}

function credentialToAssertionBody(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAssertionResponse
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      authenticatorData: bufferToBase64url(response.authenticatorData),
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : "",
    },
  }
}

function formatProviderName(provider: string) {
  if (!provider) {
    return "Social"
  }
  if (provider.toLowerCase() === "github") {
    return "GitHub"
  }
  if (provider.toLowerCase() === "google") {
    return "Google"
  }
  return provider.charAt(0).toUpperCase() + provider.slice(1)
}

function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/")
  const pad = base64.length % 4 === 0 ? "" : "=".repeat(4 - (base64.length % 4))
  const binary = atob(base64 + pad)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ""
  for (const b of bytes) {
    binary += String.fromCharCode(b)
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}
