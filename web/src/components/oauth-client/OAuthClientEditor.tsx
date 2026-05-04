import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ArrowLeft, Copy, AlertTriangle, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"

export interface OAuthClientFormData {
  name: string
  scopes: string
}

interface OAuthClientMutationResponse {
  client_secret?: string
  client?: {
    client_id?: string
    name?: string
    scopes?: string
  }
}

interface OAuthClientGetResponse {
  client: {
    client_id?: string
    name?: string
    scopes?: string
  }
}

interface OAuthClientEditorProps {
  namespace: "clients"
  id?: string
  backPath: string
  onCreate: (data: OAuthClientFormData) => Promise<OAuthClientMutationResponse>
  onGet: (id: string) => Promise<OAuthClientGetResponse>
  onUpdate: (id: string, data: OAuthClientFormData) => Promise<void>
  showCreatedClientId?: boolean
  showExistingClientId?: boolean
  showEndpointInfo?: boolean
}

export default function OAuthClientEditor({
  namespace,
  id,
  backPath,
  onCreate,
  onGet,
  onUpdate,
  showCreatedClientId = false,
  showExistingClientId = false,
  showEndpointInfo = false,
}: OAuthClientEditorProps) {
  const isNew = !id
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [bootLoading, setBootLoading] = useState(false)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [createdClientId, setCreatedClientId] = useState<string | null>(null)
  const [existingClientId, setExistingClientId] = useState("")
  const [form, setForm] = useState<OAuthClientFormData>({
    name: "",
    scopes: "read",
  })

  useEffect(() => {
    const load = async () => {
      if (isNew || !id) return

      setBootLoading(true)
      setErrorKind(null)
      try {
        const res = await onGet(id)
        const client = res.client
        if (showExistingClientId) {
          setExistingClientId(client.client_id || "")
        }
        setForm({
          name: client.name || "",
          scopes: client.scopes || "read",
        })
      } catch (error) {
        setErrorKind(getApiErrorKind(error))
      } finally {
        setBootLoading(false)
      }
    }

    void load()
  }, [id, isNew, onGet, showExistingClientId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      if (isNew) {
        const res = await onCreate(form)
        setCreatedSecret(res.client_secret || null)
        if (showCreatedClientId) {
          setCreatedClientId(res.client?.client_id || null)
        }
        toast.success(t(`${namespace}.created`))
      } else if (id) {
        await onUpdate(id, form)
        toast.success(t(`${namespace}.updated`))
        navigate(backPath)
      }
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  const copyText = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success(t("common.copied"))
  }

  const baseUrl = window.location.origin
  const nameID = `${namespace}-name`
  const scopesID = `${namespace}-scopes`

  if (createdSecret) {
    return (
      <div className="max-w-lg space-y-6">
        <Card>
          <CardContent className="space-y-4 pt-6">
            <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900 dark:bg-amber-950">
              <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />
              <p className="text-sm text-amber-800 dark:text-amber-200">{t(`${namespace}.secretWarning`)}</p>
            </div>
            {showCreatedClientId && createdClientId && (
              <div className="space-y-2">
                <Label>{t(`${namespace}.clientId`)}</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 break-all rounded-md bg-muted px-3 py-2 font-mono text-sm">
                    {createdClientId}
                  </code>
                  <Button variant="outline" size="icon" onClick={() => copyText(createdClientId)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
            <div className="space-y-2">
              <Label>{t(`${namespace}.clientSecret`)}</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 break-all rounded-md bg-muted px-3 py-2 font-mono text-sm">
                  {createdSecret}
                </code>
                <Button variant="outline" size="icon" onClick={() => copyText(createdSecret)}>
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
            </div>

            {showEndpointInfo && (
              <div className="space-y-2 rounded-lg border p-4">
                <p className="text-sm font-medium">{t("clients.machineEndpoints")}</p>
                <p className="text-xs text-muted-foreground">{t("clients.machineEndpointsDesc")}</p>
                <div className="space-y-1 font-mono text-xs text-muted-foreground">
                  <p>Token: {baseUrl}/oauth/token</p>
                  <p>Introspect: {baseUrl}/oauth/introspect</p>
                  <p>Revoke: {baseUrl}/oauth/revoke</p>
                </div>
              </div>
            )}

            <Button onClick={() => navigate(backPath)}>{t("common.back")}</Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (bootLoading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} />
  }

  return (
    <div className="max-w-lg space-y-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate(backPath)}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h1 className="text-2xl font-semibold">{isNew ? t(`${namespace}.create`) : t(`${namespace}.edit`)}</h1>
      </div>

      <Card>
        <CardContent className="pt-6">
          <form onSubmit={handleSubmit} className="space-y-4">
            {!isNew && showExistingClientId && existingClientId && (
              <div className="space-y-2">
                <Label>{t(`${namespace}.clientId`)}</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 break-all rounded-md bg-muted px-3 py-2 font-mono text-sm">
                    {existingClientId}
                  </code>
                  <Button type="button" variant="outline" size="icon" onClick={() => copyText(existingClientId)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor={nameID}>{t(`${namespace}.name`)}</Label>
              <Input id={nameID} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </div>

            <div className="space-y-2">
              <Label htmlFor={scopesID}>{t(`${namespace}.scopes`)}</Label>
              <Input id={scopesID} value={form.scopes} onChange={(e) => setForm({ ...form, scopes: e.target.value })} />
              <p className="text-xs text-muted-foreground">{t("clients.machineScopesHint")}</p>
            </div>

            {!isNew && showEndpointInfo && (
              <div className="space-y-2 rounded-lg border p-4">
                <p className="text-sm font-medium">{t("clients.machineEndpoints")}</p>
                <p className="text-xs text-muted-foreground">{t("clients.machineEndpointsDesc")}</p>
                <div className="space-y-1 font-mono text-xs text-muted-foreground">
                  <p>Token: {baseUrl}/oauth/token</p>
                  <p>Introspect: {baseUrl}/oauth/introspect</p>
                  <p>Revoke: {baseUrl}/oauth/revoke</p>
                </div>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <Button type="submit" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t(`${namespace}.save`)}
              </Button>
              <Button type="button" variant="outline" onClick={() => navigate(backPath)}>
                {t(`${namespace}.cancel`)}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
