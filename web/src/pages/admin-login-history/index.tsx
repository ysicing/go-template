import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Loader2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { loginHistoryApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import dayjs from "dayjs"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import PageErrorState from "@/components/page-error-state"

interface LoginEvent {
  id: string
  user_id: string
  username: string
  client_id: string
  app_name: string
  provider: string
  ip: string
  user_agent: string
  created_at: string
}

const PAGE_SIZE = 20

const providerVariant: Record<string, "default" | "secondary" | "outline"> = {
  local: "secondary",
  github: "default",
  google: "default",
}

export default function AdminLoginHistoryPage() {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.loginHistoryRead)
  const [events, setEvents] = useState<LoginEvent[]>([])

  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)

  const fetchEvents = useCallback(async (p: number) => {
    if (!canRead) {
      setEvents([])
      setTotal(0)
      setLoading(false)
      return
    }
    setLoading(true)
    setErrorKind(null)
    try {
      const res = await loginHistoryApi.listAll(p, PAGE_SIZE)
      setEvents(res.data.events || [])
      setTotal(res.data.total || 0)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }, [canRead, t])

  useEffect(() => { fetchEvents(page) }, [page, fetchEvents])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  const appLabel = (event: LoginEvent) => {
    if (event.app_name) return event.app_name
    if (event.client_id) return event.client_id
    return t("loginHistory.mainSite")
  }

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchEvents(page)} />
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">{t("loginHistory.adminTitle")}</h1>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("loginHistory.user")}</TableHead>
                <TableHead>{t("loginHistory.app")}</TableHead>
                <TableHead>{t("loginHistory.provider")}</TableHead>
                <TableHead>{t("loginHistory.ip")}</TableHead>
                <TableHead>{t("loginHistory.device")}</TableHead>
                <TableHead>{t("loginHistory.time")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center py-8">
                    <Loader2 className="h-5 w-5 animate-spin mx-auto text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ) : events.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                    {t("common.noData")}
                  </TableCell>
                </TableRow>
              ) : (
                events.map((e) => (
                  <TableRow key={e.id}>
                    <TableCell className="font-medium">{e.username || e.user_id}</TableCell>
                    <TableCell>{appLabel(e)}</TableCell>
                    <TableCell>
                      <Badge variant={providerVariant[e.provider] ?? "outline"}>{e.provider}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-sm">{e.ip || "-"}</TableCell>
                    <TableCell
                      className="text-sm text-muted-foreground max-w-xs truncate"
                      title={e.user_agent}
                    >
                      {e.user_agent ? (e.user_agent.length > 40 ? e.user_agent.slice(0, 40) + "…" : e.user_agent) : "-"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {dayjs(e.created_at).format("YYYY-MM-DD HH:mm")}
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
