import { useEffect, useRef, useState, useCallback } from "react"
import { useNavigate, Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { UserPlus, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { authApi } from "@/api/services"
import { getErrorMessage } from "@/api/client"
import { useAuthStore } from "@/stores/auth"
import { useAppStore } from "@/stores/app"

declare global {
  interface Window {
    turnstile?: {
      render: (container: string | HTMLElement, options: Record<string, unknown>) => string
      reset: (widgetId: string) => void
      remove: (widgetId: string) => void
    }
  }
}

export default function RegisterPage() {
  const [username, setUsername] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [turnstileSiteKey, setTurnstileSiteKey] = useState("")
  const [turnstileToken, setTurnstileToken] = useState("")
  const turnstileRef = useRef<HTMLDivElement>(null)
  const widgetIdRef = useRef<string | null>(null)
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { setUser } = useAuthStore()
  const { themeMode } = useAppStore()

  useEffect(() => {
    authApi.config().then((res) => {
      setTurnstileSiteKey(res.data.turnstile_site_key || "")
    }).catch(() => {})
  }, [])

  const renderWidget = useCallback(() => {
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

    // Check if script already loaded
    if (window.turnstile) {
      renderWidget()
      return
    }

    const script = document.createElement("script")
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
    script.async = true
    script.onload = () => renderWidget()
    document.head.appendChild(script)

    return () => {
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = null
      }
    }
  }, [turnstileSiteKey, renderWidget])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await authApi.register(username, email, password, turnstileToken || undefined)
      const { user } = res.data
      // Tokens are set in HttpOnly cookies
      setUser(user)
      if (res.data.email_verification_required) {
        navigate("/verify-email")
      } else {
        navigate("/")
      }
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.reset(widgetIdRef.current)
        setTurnstileToken("")
      }
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <UserPlus className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold">{t("app.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("register.title")}</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">{t("register.username")}</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">{t("register.email")}</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{t("register.password")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
              <p className="text-xs text-muted-foreground">{t("register.passwordHint")}</p>
            </div>
            {turnstileSiteKey && (
              <div ref={turnstileRef} className="flex justify-center" />
            )}
            <Button type="submit" className="w-full" disabled={loading || (!!turnstileSiteKey && !turnstileToken)}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("register.submit")}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              {t("register.hasAccount")}{" "}
              <Link to="/login" className="text-primary hover:underline">
                {t("register.login")}
              </Link>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
