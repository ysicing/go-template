import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Trash2, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { sessionApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import dayjs from "dayjs"
import PageErrorState from "@/components/page-error-state"

interface Session {
  id: string
  ip: string
  user_agent: string
  last_used_at: string
  created_at: string
}

const PAGE_SIZE = 10

export default function SessionsPage({ hideTitle }: { hideTitle?: boolean }) {
  const { t } = useTranslation()
  const [sessions, setSessions] = useState<Session[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [revokingAll, setRevokingAll] = useState(false)

  const fetchSessions = useCallback(async (p: number) => {
    setLoading(true)
    setErrorKind(null)
    try {
      const res = await sessionApi.list(p, PAGE_SIZE)
      setSessions(res.data.sessions || [])
      setTotal(res.data.total || 0)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => { fetchSessions(page) }, [page, fetchSessions])

  const handleRevoke = async (id: string) => {
    setRevoking(id)
    try {
      await sessionApi.revoke(id)
      toast.success(t("sessions.revoked"))
      void fetchSessions(page)
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setRevoking(null)
    }
  }

  const handleRevokeAll = async () => {
    setRevokingAll(true)
    try {
      await sessionApi.revokeAll()
      toast.success(t("sessions.allRevoked"))
      setPage(1)
      void fetchSessions(1)
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setRevokingAll(false)
    }
  }

  const truncateUA = (ua: string) => {
    if (!ua) return t("sessions.unknown")
    if (ua.length > 60) return ua.slice(0, 60) + "…"
    return ua
  }

  const totalPages = Math.ceil(total / PAGE_SIZE)

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchSessions(page)} compact={hideTitle} />
  }

  return (
    <div className="space-y-6">
      {!hideTitle && (
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">{t("sessions.title")}</h1>
          {total > 0 && (
            <Button variant="destructive" size="sm" onClick={handleRevokeAll} disabled={revokingAll}>
              {revokingAll && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("sessions.revokeAll")}
            </Button>
          )}
        </div>
      )}
      {hideTitle && total > 0 && (
        <div className="flex justify-end">
          <Button variant="destructive" size="sm" onClick={handleRevokeAll} disabled={revokingAll}>
            {revokingAll && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("sessions.revokeAll")}
          </Button>
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("sessions.ip")}</TableHead>
                <TableHead>{t("sessions.userAgent")}</TableHead>
                <TableHead>{t("sessions.lastUsed")}</TableHead>
                <TableHead>{t("sessions.createdAt")}</TableHead>
                <TableHead className="w-16" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center py-8">
                    <Loader2 className="h-5 w-5 animate-spin mx-auto text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ) : sessions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                    {t("sessions.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                sessions.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-mono text-sm">{s.ip || "-"}</TableCell>
                    <TableCell className="text-sm text-muted-foreground max-w-xs truncate" title={s.user_agent}>
                      {truncateUA(s.user_agent)}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {s.last_used_at ? dayjs(s.last_used_at).format("YYYY-MM-DD HH:mm") : "-"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {dayjs(s.created_at).format("YYYY-MM-DD HH:mm")}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleRevoke(s.id)}
                        disabled={revoking === s.id}
                      >
                        {revoking === s.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Trash2 className="h-4 w-4 text-destructive" />
                        )}
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
