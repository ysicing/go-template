import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ArrowLeft, Copy, Loader2, Pencil } from "lucide-react"

import { userAppApi, statsApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Label } from "@/components/ui/label"

interface AppDetail {
  id?: string
  name: string
  client_id: string
  redirect_uris: string
  grant_types: string
  scopes: string
  require_consent?: boolean
}

interface AppStat {
  client_id: string
  user_count: number
  login_count: number
}

export default function AppViewPage() {
  const { id } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [app, setApp] = useState<AppDetail | null>(null)
  const [userCount, setUserCount] = useState(0)
  const [loginCount, setLoginCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [revealedSecret, setRevealedSecret] = useState("")
  const [rotateLoading, setRotateLoading] = useState(false)

  const applyAppStat = (clientID: string, appStats: AppStat[]) => {
    const stat = appStats.find((item) => item.client_id === clientID)
    if (stat) {
      setUserCount(stat.user_count)
      setLoginCount(stat.login_count)
      return
    }
    setUserCount(0)
    setLoginCount(0)
  }

  useEffect(() => {
    if (!id) {
      return
    }

    const load = async () => {
      setLoading(true)
      setErrorKind(null)

      try {
        const [appRes, statsRes] = await Promise.all([userAppApi.get(id), statsApi.user()])
        const client = appRes.data.application || appRes.data.client
        setApp(client)
        applyAppStat(client.client_id, (statsRes.data.app_stats || []) as AppStat[])
      } catch (error) {
        setErrorKind(getApiErrorKind(error))
        toast.error(getErrorMessage(error, t("common.error")))
      } finally {
        setLoading(false)
      }
    }

    void load()
  }, [id, t])

  const copyText = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success(t("common.copied"))
  }

  const handleRotateSecret = async () => {
    if (!id) return

    setRotateLoading(true)
    try {
      const res = await userAppApi.rotateSecret(id)
      setRevealedSecret(String(res.data.client_secret || ""))
      toast.success(t("apps.secretRotated"))
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setRotateLoading(false)
    }
  }

  if (loading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} />
  }

  if (!app) {
    return <PageErrorState kind="not_found" onRetry={() => window.location.reload()} />
  }

  const baseUrl = window.location.origin

  return (
    <div className="max-w-3xl space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate("/uauth/apps")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-2xl font-semibold">{app.name}</h1>
        </div>
        <Button variant="outline" size="sm" onClick={() => navigate(`/uauth/apps/${id}`)}>
          <Pencil className="mr-2 h-4 w-4" />
          {t("apps.edit")}
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6 space-y-4">
          <div className="space-y-2">
            <Label>{t("apps.clientId")}</Label>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded-md bg-muted px-3 py-2 text-sm font-mono break-all">
                {app.client_id}
              </code>
              <Button variant="outline" size="icon" onClick={() => copyText(app.client_id)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("apps.redirectUris")}</Label>
            <div className="rounded-md bg-muted px-3 py-2 text-sm font-mono break-all whitespace-pre-wrap">
              {app.redirect_uris || "-"}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{t("apps.grantTypes")}</Label>
              <div className="text-sm text-muted-foreground">{app.grant_types || "-"}</div>
            </div>
            <div className="space-y-2">
              <Label>{t("apps.scopes")}</Label>
              <div className="flex flex-wrap gap-1">
                {app.scopes?.split(/[, ]+/).filter(Boolean).map((scope) => (
                  <Badge key={scope} variant="secondary" className="text-xs">{scope}</Badge>
                ))}
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("apps.consentPolicy")}</Label>
            <div className="rounded-lg border p-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">{t("apps.requireConsent")}</p>
                  <p className="text-xs text-muted-foreground">{t("apps.requireConsentDesc")}</p>
                </div>
                <Badge variant={app.require_consent ? "default" : "secondary"}>
                  {app.require_consent ? t("common.enabled") : t("common.disabled")}
                </Badge>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg border p-3 text-center">
              <p className="text-2xl font-semibold">{userCount}</p>
              <p className="text-xs text-muted-foreground">{t("apps.users")}</p>
            </div>
            <div className="rounded-lg border p-3 text-center">
              <p className="text-2xl font-semibold">{loginCount}</p>
              <p className="text-xs text-muted-foreground">{t("apps.logins")}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="pt-6 space-y-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-sm font-medium">{t("apps.clientSecret")}</p>
              <p className="text-xs text-muted-foreground">{t("apps.secretRevealHelp")}</p>
            </div>
            <Button variant="outline" size="sm" onClick={handleRotateSecret} disabled={rotateLoading}>
              {rotateLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("apps.rotateSecret")}
            </Button>
          </div>

          <div className="space-y-1 text-xs text-muted-foreground font-mono">
            <p>Discovery: {baseUrl}/.well-known/openid-configuration</p>
            <p>Authorization: {baseUrl}/authorize</p>
            <p>Token: {baseUrl}/oauth/token</p>
          </div>

          {revealedSecret && (
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded-md bg-muted px-3 py-2 text-sm font-mono break-all">
                {revealedSecret}
              </code>
              <Button variant="outline" size="icon" onClick={() => copyText(revealedSecret)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
