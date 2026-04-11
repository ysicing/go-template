import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Plus, Pencil, Trash2, Copy } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"

export interface OAuthClientListItem {
  id: string
  name: string
  client_id: string
  redirect_uris: string
  grant_types: string
  scopes: string
}

interface OAuthClientStat {
  client_id: string
  login_count: number
  user_count: number
}

interface OAuthClientListPageProps {
  title: string
  createLabel: string
  newPath?: string
  listColumns: {
    name: string
    clientId: string
    scopes: string
    actions: string
    users?: string
    logins?: string
  }
  deleteDialog: {
    title: string
    description: string
    successMessage: string
  }
  fetchList: (page: number, pageSize: number) => Promise<{
    clients: OAuthClientListItem[]
    total: number
  }>
  deleteItem?: (id: string) => Promise<void>
  rowClickPath?: (id: string) => string
  editPath?: (id: string) => string
  enableStats?: boolean
  fetchStats?: () => Promise<OAuthClientStat[]>
}

export default function OAuthClientListPage({
  title,
  createLabel,
  newPath,
  listColumns,
  deleteDialog,
  fetchList,
  deleteItem,
  rowClickPath,
  editPath,
  enableStats = false,
  fetchStats,
}: OAuthClientListPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [items, setItems] = useState<OAuthClientListItem[]>([])
  const [, setTotal] = useState(0)
  const [page] = useState(1)
  const [loading, setLoading] = useState(false)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<OAuthClientListItem | null>(null)
  const [statMap, setStatMap] = useState<Record<string, OAuthClientStat>>({})
  const canCreate = !!newPath
  const canEdit = !!editPath
  const canDelete = !!deleteItem
  const showActions = canEdit || canDelete
  const emptyColSpan = 3 + (enableStats ? 2 : 0) + (showActions ? 1 : 0)

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      setErrorKind(null)
      try {
        const listRes = await fetchList(page, 20)
        setItems(listRes.clients)
        setTotal(listRes.total)

        if (enableStats && fetchStats) {
          const stats = await fetchStats()
          const nextMap: Record<string, OAuthClientStat> = {}
          for (const stat of stats) {
            nextMap[stat.client_id] = stat
          }
          setStatMap(nextMap)
        }
      } catch (error) {
        setErrorKind(getApiErrorKind(error))
      } finally {
        setLoading(false)
      }
    }

    load()
  }, [enableStats, fetchList, fetchStats, page])

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      if (!deleteItem) return
      await deleteItem(deleteTarget.id)
      toast.success(deleteDialog.successMessage)
      setDeleteTarget(null)

      const listRes = await fetchList(page, 20)
      setItems(listRes.clients)
      setTotal(listRes.total)

      if (enableStats && fetchStats) {
        const stats = await fetchStats()
        const nextMap: Record<string, OAuthClientStat> = {}
        for (const stat of stats) {
          nextMap[stat.client_id] = stat
        }
        setStatMap(nextMap)
      }
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    }
  }

  const copyId = (id: string) => {
    navigator.clipboard.writeText(id)
    toast.success(t("common.copied"))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">{title}</h1>
        {canCreate && (
          <Button onClick={() => navigate(newPath)}>
            <Plus className="mr-2 h-4 w-4" />
            {createLabel}
          </Button>
        )}
      </div>

      {errorKind ? (
        <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} />
      ) : (

        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{listColumns.name}</TableHead>
                  <TableHead>{listColumns.clientId}</TableHead>
                  <TableHead>{listColumns.scopes}</TableHead>
                  {enableStats && (
                    <>
                      <TableHead className="text-center">{listColumns.users}</TableHead>
                      <TableHead className="text-center">{listColumns.logins}</TableHead>
                    </>
                  )}
                  {showActions && <TableHead className="w-28">{listColumns.actions}</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow
                    key={item.id}
                    className={rowClickPath ? "cursor-pointer" : undefined}
                    onClick={() => {
                      if (rowClickPath) {
                        navigate(rowClickPath(item.id))
                      }
                    }}
                    >
                      <TableCell className="font-medium">{item.name}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{item.client_id}</code>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          onClick={(e) => {
                            e.stopPropagation()
                            copyId(item.client_id)
                          }}
                        >
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {item.scopes?.split(/[, ]+/).filter(Boolean).map((scope) => (
                          <Badge key={scope} variant="secondary" className="text-xs">{scope}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    {enableStats && (
                      <>
                        <TableCell className="text-center text-muted-foreground">
                          {statMap[item.client_id]?.user_count ?? 0}
                        </TableCell>
                        <TableCell className="text-center text-muted-foreground">
                          {statMap[item.client_id]?.login_count ?? 0}
                        </TableCell>
                      </>
                    )}
                    {showActions && (
                      <TableCell onClick={(e) => e.stopPropagation()}>
                        <div className="flex gap-1">
                          {canEdit && editPath && (
                            <Button variant="ghost" size="icon" onClick={() => navigate(editPath(item.id))}>
                              <Pencil className="h-4 w-4" />
                            </Button>
                          )}
                          {canDelete && (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="text-destructive hover:text-destructive"
                              onClick={() => setDeleteTarget(item)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {!loading && items.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={emptyColSpan} className="py-8 text-center text-muted-foreground">
                      {t("common.noData")}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {canDelete && (
        <Dialog
          open={!!deleteTarget}
          onOpenChange={(open) => {
            if (!open) setDeleteTarget(null)
          }}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{deleteDialog.title}</DialogTitle>
              <DialogDescription>{deleteDialog.description}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>
                {t("common.cancel")}
              </Button>
              <Button variant="destructive" onClick={handleDelete}>
                {t("common.confirm")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  )
}
