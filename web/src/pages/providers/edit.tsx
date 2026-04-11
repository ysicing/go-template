import { useEffect, useState } from "react"
import { Navigate, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ArrowLeft, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { adminProviderApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import PageErrorState from "@/components/page-error-state"

export default function ProviderEditPage() {
  const { id } = useParams()
  const isNew = !id
  const { t } = useTranslation()
  const navigate = useNavigate()
  const canWrite = useHasPermission(adminPermissions.providersWrite)
  const [loading, setLoading] = useState(false)
  const [bootLoading, setBootLoading] = useState(false)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [form, setForm] = useState({
    name: "",
    client_id: "",
    client_secret: "",
    redirect_url: "",
    enabled: true,
  })

  useEffect(() => {
    if (!isNew && canWrite) {
      setBootLoading(true)
      setErrorKind(null)
      adminProviderApi.get(id).then((res) => {
        const p = res.data.provider
        setForm({
          name: p.name || "",
          client_id: p.client_id || "",
          client_secret: "",
          redirect_url: p.redirect_url || "",
          enabled: p.enabled ?? true,
        })
      }).catch((error) => {
        setErrorKind(getApiErrorKind(error))
      }).finally(() => {
        setBootLoading(false)
      })
    }
  }, [id, isNew, canWrite])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      if (isNew) {
        await adminProviderApi.create(form)
        toast.success(t("providers.created"))
      } else {
        await adminProviderApi.update(id!, form)
        toast.success(t("providers.updated"))
      }
      navigate("/admin/providers")
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  if (!canWrite) {
    return <Navigate to="/admin/providers" replace />
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
        <Button variant="ghost" size="icon" onClick={() => navigate("/admin/providers")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h1 className="text-2xl font-semibold">{isNew ? t("providers.create") : t("providers.edit")}</h1>
      </div>
      <Card>
        <CardContent className="pt-6">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label>{t("providers.name")}</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="github / google"
                required
                disabled={!isNew}
              />
            </div>
            <div className="space-y-2">
              <Label>{t("providers.clientId")}</Label>
              <Input value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })} required />
            </div>
            <div className="space-y-2">
              <Label>{t("providers.clientSecret")}</Label>
              <Input
                type="password"
                value={form.client_secret}
                onChange={(e) => setForm({ ...form, client_secret: e.target.value })}
                placeholder={isNew ? "" : "Leave empty to keep current"}
                required={isNew}
              />
            </div>
            <div className="space-y-2">
              <Label>{t("providers.redirectUrl")}</Label>
              <Input value={form.redirect_url} onChange={(e) => setForm({ ...form, redirect_url: e.target.value })} />
            </div>
            <div className="flex items-center gap-3">
              <Switch checked={form.enabled} onCheckedChange={(v) => setForm({ ...form, enabled: v })} />
              <Label>{t("providers.enabled")}</Label>
            </div>
            <div className="flex gap-3 pt-2">
              <Button type="submit" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t("providers.save")}
              </Button>
              <Button type="button" variant="outline" onClick={() => navigate("/admin/providers")}>
                {t("providers.cancel")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
