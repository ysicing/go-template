import { useCallback, useEffect, useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Check, KeyRound, Loader2, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import PageErrorState from "@/components/page-error-state"
import { authApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { redirectToSameOrigin } from "@/lib/navigation"

type ConsentContext = {
  client?: {
    id?: string
    name?: string
  }
  scopes?: string[]
  branding?: {
    display_name?: string
    headline?: string
    logo_url?: string
    primary_color?: string
  } | null
  requires_consent?: boolean
}

export default function ConsentPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [context, setContext] = useState<ConsentContext | null>(null)
  const [submitting, setSubmitting] = useState<"approve" | "deny" | null>(null)

  const authRequestID = searchParams.get("id") ?? ""

  const loadContext = useCallback(async () => {
    if (!authRequestID) {
      setErrorKind("not_found")
      setLoading(false)
      return
    }

    setLoading(true)
    setErrorKind(null)
    try {
      const res = await authApi.oidcConsentContext(authRequestID)
      setContext(res.data ?? null)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
    } finally {
      setLoading(false)
    }
  }, [authRequestID])

  useEffect(() => {
    void loadContext()
  }, [loadContext])

  const brandStyle = context?.branding?.primary_color
    ? { backgroundColor: context.branding.primary_color, borderColor: context.branding.primary_color }
    : undefined

  const handleRedirect = (redirect?: string) => {
    if (!redirect) {
      navigate("/login", { replace: true })
      return
    }
    if (!redirectToSameOrigin(redirect)) {
      navigate("/login", { replace: true })
    }
  }

  const handleApprove = async () => {
    if (!authRequestID) return
    setSubmitting("approve")
    try {
      const res = await authApi.oidcConsentApprove(authRequestID)
      handleRedirect(res.data.redirect)
    } catch (error) {
      toast.error(getErrorMessage(error, t("consent.approveError")))
    } finally {
      setSubmitting(null)
    }
  }

  const handleDeny = async () => {
    if (!authRequestID) return
    setSubmitting("deny")
    try {
      const res = await authApi.oidcConsentDeny(authRequestID)
      handleRedirect(res.data.redirect)
    } catch (error) {
      toast.error(getErrorMessage(error, t("consent.denyError")))
    } finally {
      setSubmitting(null)
    }
  }

  if (loading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind || !context) {
    return <PageErrorState kind={errorKind ?? "unknown"} onRetry={() => void loadContext()} />
  }

  const displayName = context.branding?.display_name || t("app.title")
  const headline = context.branding?.headline || t("consent.title")
  const scopes = context.scopes ?? []

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-lg">
        <CardHeader className="space-y-4 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center overflow-hidden rounded-xl bg-primary text-primary-foreground" style={brandStyle}>
            {context.branding?.logo_url ? (
              <img src={context.branding.logo_url} alt={displayName} className="h-full w-full object-cover" />
            ) : (
              <KeyRound className="h-6 w-6" />
            )}
          </div>
          <div className="space-y-1">
            <CardTitle>{displayName}</CardTitle>
            <CardDescription>{headline}</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <p className="text-sm font-medium">{context.client?.name || t("consent.unknownApp")}</p>
            <p className="text-sm text-muted-foreground">{t("consent.requestedAccess")}</p>
          </div>

          <div className="flex flex-wrap gap-2">
            {scopes.map((scope) => (
              <Badge key={scope} variant="secondary">{scope}</Badge>
            ))}
          </div>

          <div className="flex gap-3">
            <Button className="flex-1" onClick={() => void handleApprove()} disabled={submitting !== null} style={brandStyle}>
              {submitting === "approve" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Check className="mr-2 h-4 w-4" />}
              {t("consent.allow")}
            </Button>
            <Button variant="outline" className="flex-1" onClick={() => void handleDeny()} disabled={submitting !== null}>
              {submitting === "deny" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <X className="mr-2 h-4 w-4" />}
              {t("consent.cancel")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
