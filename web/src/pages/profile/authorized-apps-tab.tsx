import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import dayjs from "dayjs"
import { Loader2 } from "lucide-react"
import { userApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface AuthorizedApp {
  id: string
  client_id: string
  client_name: string
  scopes: string
  granted_at: string
}

const PAGE_SIZE = 10

export function AuthorizedAppsTab() {
  const { t } = useTranslation()
  const [apps, setApps] = useState<AuthorizedApp[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [revokingId, setRevokingId] = useState<string | null>(null)

  const loadApps = useCallback(async (nextPage: number) => {
    setLoading(true)
    setErrorKind(null)
    try {
      const response = await userApi.listAuthorizedApps(nextPage, PAGE_SIZE)
      setApps(response.data.apps || [])
      setTotal(response.data.total || 0)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadApps(page)
  }, [loadApps, page])

  const handleRevoke = async (id: string) => {
    setRevokingId(id)
    try {
      await userApi.revokeAuthorizedApp(id)
      toast.success(t("profile.authorizedAppsRevoked"))
      void loadApps(page)
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setRevokingId(null)
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE)

  if (errorKind) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.authorizedApps")}</CardTitle>
          <CardDescription>{t("profile.authorizedAppsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <PageErrorState kind={errorKind} onRetry={() => void loadApps(page)} compact />
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.authorizedApps")}</CardTitle>
          <CardDescription>{t("profile.authorizedAppsDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("profile.authorizedAppsApp")}</TableHead>
                <TableHead>{t("profile.authorizedAppsClientId")}</TableHead>
                <TableHead>{t("profile.authorizedAppsScopes")}</TableHead>
                <TableHead>{t("profile.authorizedAppsGrantedAt")}</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center">
                    <Loader2 className="mx-auto h-5 w-5 animate-spin text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ) : apps.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                    {t("profile.authorizedAppsEmpty")}
                  </TableCell>
                </TableRow>
              ) : (
                apps.map((app) => (
                  <TableRow key={app.id}>
                    <TableCell className="font-medium">{app.client_name || app.client_id}</TableCell>
                    <TableCell className="font-mono text-sm text-muted-foreground">{app.client_id}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{app.scopes || "-"}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {app.granted_at ? dayjs(app.granted_at).format("YYYY-MM-DD HH:mm") : "-"}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void handleRevoke(app.id)}
                        disabled={revokingId === app.id}
                      >
                        {revokingId === app.id && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t("profile.authorizedAppsRevoke")}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{t("common.total", { count: total })}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              ←
            </Button>
            <span className="flex items-center px-2 text-sm">{page} / {totalPages}</span>
            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              →
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

export default AuthorizedAppsTab
