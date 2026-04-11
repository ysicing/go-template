import { useEffect, useState, useCallback } from "react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Plus, Pencil, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { adminProviderApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import PageErrorState from "@/components/page-error-state"

interface SocialProvider {
  id: string
  name: string
  client_id: string
  redirect_url: string
  enabled: boolean
}

export default function ProvidersPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const canRead = useHasPermission(adminPermissions.providersRead)
  const canWrite = useHasPermission(adminPermissions.providersWrite)
  const [providers, setProviders] = useState<SocialProvider[]>([])
  const [loading, setLoading] = useState(false)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<SocialProvider | null>(null)

  const fetchProviders = useCallback(async () => {
    setLoading(true)
    setErrorKind(null)
    try {
      const res = await adminProviderApi.list()
      setProviders(res.data.providers || [])
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (canRead) {
      fetchProviders()
    }
  }, [canRead, fetchProviders])

  const handleDelete = async () => {
    if (!deleteTarget || !canWrite) return
    try {
      await adminProviderApi.delete(deleteTarget.id)
      toast.success(t("providers.deleted"))
      setDeleteTarget(null)
      fetchProviders()
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    }
  }

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchProviders()} />
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">{t("providers.title")}</h1>
        {canWrite && (
          <Button onClick={() => navigate("/admin/providers/new")}>
            <Plus className="mr-2 h-4 w-4" />
            {t("providers.create")}
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("providers.name")}</TableHead>
                <TableHead>{t("providers.clientId")}</TableHead>
                <TableHead>{t("providers.redirectUrl")}</TableHead>
                <TableHead>{t("providers.enabled")}</TableHead>
                {canWrite && <TableHead className="w-28">{t("users.actions")}</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {providers.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium capitalize">{p.name}</TableCell>
                  <TableCell>
                    <code className="text-xs bg-muted px-1.5 py-0.5 rounded">{p.client_id}</code>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                    {p.redirect_url || "—"}
                  </TableCell>
                  <TableCell>
                    {p.enabled ? (
                      <Badge className="bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400">
                        {t("users.yes")}
                      </Badge>
                    ) : (
                      <Badge variant="secondary">{t("users.no")}</Badge>
                    )}
                  </TableCell>
                  {canWrite && (
                    <TableCell>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => navigate(`/admin/providers/${p.id}`)}>
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-destructive hover:text-destructive"
                          onClick={() => setDeleteTarget(p)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
              {!loading && providers.length === 0 && (
                <TableRow>
                  <TableCell colSpan={canWrite ? 5 : 4} className="text-center text-muted-foreground py-8">
                    {t("common.noData")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {canWrite && (
        <Dialog open={!!deleteTarget} onOpenChange={() => setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("providers.delete")}</DialogTitle>
              <DialogDescription>{t("providers.deleteConfirm")}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>{t("common.cancel")}</Button>
              <Button variant="destructive" onClick={handleDelete}>{t("common.confirm")}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  )
}
