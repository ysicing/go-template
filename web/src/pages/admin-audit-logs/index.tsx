import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import dayjs from "dayjs"
import { Loader2, Search, ShieldAlert } from "lucide-react"
import { toast } from "sonner"
import { auditLogApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import PageErrorState from "@/components/page-error-state"

interface AuditLogRow {
  id: string
  user_id: string
  username: string
  action: string
  resource: string
  resource_id: string
  client_id: string
  ip: string
  user_agent: string
  detail: string
  status: string
  source: string
  created_at: string
}

type AuditLogFilters = {
  keyword: string
  user_id: string
  action: string
  resource: string
  source: string
  status: string
  ip: string
}

const PAGE_SIZE = 20

const defaultFilters: AuditLogFilters = {
  keyword: "",
  user_id: "",
  action: "",
  resource: "",
  source: "",
  status: "",
  ip: "",
}

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  success: "secondary",
  failure: "destructive",
}

const sourceVariant: Record<string, "default" | "secondary" | "outline"> = {
  admin: "default",
  api: "secondary",
  web: "outline",
  cli: "outline",
  system: "outline",
}

function truncateMiddle(value: string, max = 52) {
  if (!value || value.length <= max) return value || "-"
  return `${value.slice(0, max - 16)}...${value.slice(-12)}`
}

export default function AdminAuditLogsPage() {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.loginHistoryRead)
  const [logs, setLogs] = useState<AuditLogRow[]>([])
  const [filters, setFilters] = useState<AuditLogFilters>(defaultFilters)
  const [appliedFilters, setAppliedFilters] = useState<AuditLogFilters>(defaultFilters)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)

  const fetchLogs = useCallback(async (currentPage: number, currentFilters: AuditLogFilters) => {
    if (!canRead) {
      setLogs([])
      setTotal(0)
      setLoading(false)
      return
    }

    setLoading(true)
    setErrorKind(null)
    try {
      const res = await auditLogApi.list({
        page: currentPage,
        page_size: PAGE_SIZE,
        ...currentFilters,
      })
      setLogs(res.data.logs || [])
      setTotal(res.data.total || 0)
    } catch (error) {
      const kind = getApiErrorKind(error)
      setErrorKind(kind)
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }, [canRead, t])

  useEffect(() => {
    void fetchLogs(page, appliedFilters)
  }, [page, appliedFilters, fetchLogs])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const updateFilter = <K extends keyof AuditLogFilters>(key: K, value: AuditLogFilters[K]) => {
    setFilters((prev) => ({ ...prev, [key]: value }))
  }

  const applyFilters = () => {
    setPage(1)
    setAppliedFilters(filters)
  }

  const resetFilters = () => {
    setFilters(defaultFilters)
    setAppliedFilters(defaultFilters)
    setPage(1)
  }

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchLogs(page, appliedFilters)} />
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="rounded-2xl bg-primary/10 p-2 text-primary">
          <ShieldAlert className="h-5 w-5" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold">{t("auditLogs.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("auditLogs.subtitle")}</p>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-4">
          <CardTitle>{t("auditLogs.filters")}</CardTitle>
          <CardDescription>{t("auditLogs.filtersDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.keyword")}</label>
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-9"
                  value={filters.keyword}
                  onChange={(e) => updateFilter("keyword", e.target.value)}
                  placeholder={t("auditLogs.keywordPlaceholder")}
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.userId")}</label>
              <Input value={filters.user_id} onChange={(e) => updateFilter("user_id", e.target.value)} placeholder={t("auditLogs.userIdPlaceholder")} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.action")}</label>
              <Input value={filters.action} onChange={(e) => updateFilter("action", e.target.value)} placeholder={t("auditLogs.actionPlaceholder")} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.resource")}</label>
              <Input value={filters.resource} onChange={(e) => updateFilter("resource", e.target.value)} placeholder={t("auditLogs.resourcePlaceholder")} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.source")}</label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                value={filters.source}
                onChange={(e) => updateFilter("source", e.target.value)}
              >
                <option value="">{t("auditLogs.all")}</option>
                {["admin", "api", "web", "cli", "system"].map((source) => (
                  <option key={source} value={source}>{source}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.status")}</label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                value={filters.status}
                onChange={(e) => updateFilter("status", e.target.value)}
              >
                <option value="">{t("auditLogs.all")}</option>
                <option value="success">{t("auditLogs.success")}</option>
                <option value="failure">{t("auditLogs.failure")}</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("auditLogs.ip")}</label>
              <Input value={filters.ip} onChange={(e) => updateFilter("ip", e.target.value)} placeholder={t("auditLogs.ipPlaceholder")} />
            </div>
            <div className="flex items-end gap-2">
              <Button onClick={applyFilters}>{t("auditLogs.apply")}</Button>
              <Button variant="outline" onClick={resetFilters}>{t("auditLogs.reset")}</Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="space-y-4 p-0">
          <div className="flex items-center justify-between border-b px-6 py-4">
            <div className="text-sm text-muted-foreground">{t("common.total", { count: total })}</div>
          </div>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("auditLogs.time")}</TableHead>
                  <TableHead>{t("auditLogs.user")}</TableHead>
                  <TableHead>{t("auditLogs.action")}</TableHead>
                  <TableHead>{t("auditLogs.resource")}</TableHead>
                  <TableHead>{t("auditLogs.source")}</TableHead>
                  <TableHead>{t("auditLogs.status")}</TableHead>
                  <TableHead>{t("auditLogs.ip")}</TableHead>
                  <TableHead>{t("auditLogs.userAgent")}</TableHead>
                  <TableHead>{t("auditLogs.detail")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-10 text-center">
                      <Loader2 className="mx-auto h-5 w-5 animate-spin text-muted-foreground" />
                    </TableCell>
                  </TableRow>
                ) : logs.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-10 text-center text-muted-foreground">
                      {t("common.noData")}
                    </TableCell>
                  </TableRow>
                ) : (
                  logs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                        {dayjs(log.created_at).format("YYYY-MM-DD HH:mm:ss")}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="font-medium">{log.username || "-"}</div>
                          <div className="font-mono text-xs text-muted-foreground">{truncateMiddle(log.user_id, 32)}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-xs">{log.action}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="font-medium">{log.resource}</div>
                          <div className="font-mono text-xs text-muted-foreground">{truncateMiddle(log.resource_id, 28)}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={sourceVariant[log.source] ?? "outline"}>{log.source || "-"}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant[log.status] ?? "outline"}>{log.status || "-"}</Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{log.ip || "-"}</TableCell>
                      <TableCell className="max-w-[240px] text-xs text-muted-foreground" title={log.user_agent}>
                        {truncateMiddle(log.user_agent, 48)}
                      </TableCell>
                      <TableCell className="max-w-[340px] text-xs text-muted-foreground" title={log.detail}>
                        {truncateMiddle(log.detail, 96)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{page} / {totalPages}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
              {t("common.prev")}
            </Button>
            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((current) => current + 1)}>
              {t("common.next")}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
