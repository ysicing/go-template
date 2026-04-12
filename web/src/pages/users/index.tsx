import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Trash2, Plus, Copy } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { adminUserApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import dayjs from "dayjs"
import PageErrorState from "@/components/page-error-state"
import { GitHubIcon, GoogleIcon } from "@/components/icons"

interface User {
  id: string
  username: string
  email: string
  provider: string
  social_providers: string[]
  is_admin: boolean
  created_at: string
}

interface CurrentUser {
  id: string
}

const emptyForm = { username: "", email: "", is_admin: false }

export default function UsersPage({ currentUser }: { currentUser?: CurrentUser }) {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.usersRead)
  const canWrite = useHasPermission(adminPermissions.usersWrite)
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [creating, setCreating] = useState(false)
  const [generatedPassword, setGeneratedPassword] = useState("")

  const fetchUsers = useCallback(async (p: number) => {
    setLoading(true)
    setErrorKind(null)
    try {
      const res = await adminUserApi.list(p, 20)
      setUsers(res.data.users || [])
      setTotal(res.data.total)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchUsers(page) }, [page, fetchUsers])

  const totalPages = Math.ceil(total / 20)

  const getSocialProviderBadge = (provider: string) => {
    const normalized = provider.toLowerCase()
    if (normalized === "github") {
      return { icon: <GitHubIcon className="h-3.5 w-3.5" />, label: "GitHub" }
    }
    if (normalized === "google") {
      return { icon: <GoogleIcon className="h-3.5 w-3.5" />, label: "Google" }
    }
    return { icon: null, label: provider }
  }

  const handleDelete = async () => {
    if (!deleteTarget || !canWrite) return
    try {
      await adminUserApi.delete(deleteTarget.id)
      toast.success(t("users.deleted"))
      setDeleteTarget(null)
      fetchUsers(page)
    } catch (err) {
      toast.error(getErrorMessage(err, t("common.error")))
    }
  }

  const handleCreate = async () => {
    if (!canWrite) return
    setCreating(true)
    try {
      const res = await adminUserApi.create(form)
      setShowCreate(false)
      setForm(emptyForm)
      setGeneratedPassword(res.data.password)
      fetchUsers(page)
    } catch (err) {
      toast.error(getErrorMessage(err, t("common.error")))
    } finally {
      setCreating(false)
    }
  }

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchUsers(page)} />
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">{t("users.title")}</h1>
        {canWrite && (
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t("users.create")}
          </Button>
        )}
      </div>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("users.username")}</TableHead>
                <TableHead>{t("users.email")}</TableHead>
                <TableHead>{t("users.provider")}</TableHead>
                <TableHead>{t("users.socialProviders")}</TableHead>
                <TableHead>{t("users.admin")}</TableHead>
                <TableHead>{t("users.createdAt")}</TableHead>
                <TableHead className="w-20">{t("users.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.username}</TableCell>
                  <TableCell>{u.email}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{u.provider}</Badge>
                  </TableCell>
                  <TableCell>
                    {u.social_providers?.length ? (
                      <div className="flex flex-wrap gap-2">
                        {u.social_providers.map((provider) => {
                          const badge = getSocialProviderBadge(provider)
                          return (
                            <Badge key={`${u.id}-${provider}`} variant="outline" className="gap-1.5">
                              {badge.icon}
                              <span>{badge.label}</span>
                            </Badge>
                          )
                        })}
                      </div>
                    ) : (
                      <span className="text-muted-foreground text-sm">{t("users.noSocialProviders")}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {u.is_admin ? (
                      <Badge>{t("users.yes")}</Badge>
                    ) : (
                      <span className="text-muted-foreground text-sm">{t("users.no")}</span>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {dayjs(u.created_at).format("YYYY-MM-DD HH:mm")}
                  </TableCell>
                  <TableCell>
                    {canWrite && u.id !== currentUser?.id && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive"
                        onClick={() => setDeleteTarget(u)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
              {!loading && users.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                    {t("common.noData")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Pagination */}
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

      {/* Create dialog */}
      <Dialog open={showCreate && canWrite} onOpenChange={(open) => { if (!open) { setShowCreate(false); setForm(emptyForm) } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.create")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>{t("users.username")}</Label>
              <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label>{t("users.email")}</Label>
              <Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
            </div>
            <div className="flex items-center justify-between">
              <Label>{t("users.admin")}</Label>
              <Switch checked={form.is_admin} onCheckedChange={(v) => setForm({ ...form, is_admin: v })} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowCreate(false); setForm(emptyForm) }}>{t("common.cancel")}</Button>
            <Button onClick={handleCreate} disabled={creating || !form.username || !form.email || !canWrite}>{t("common.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <Dialog open={!!deleteTarget && canWrite} onOpenChange={() => setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.delete")}</DialogTitle>
            <DialogDescription>{t("users.deleteConfirm")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={handleDelete}>{t("common.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Password reveal dialog */}
      <Dialog open={!!generatedPassword} onOpenChange={() => setGeneratedPassword("")}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.created")}</DialogTitle>
            <DialogDescription>{t("users.passwordWarning")}</DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 py-2">
            <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono select-all">{generatedPassword}</code>
            <Button variant="ghost" size="icon" onClick={() => { navigator.clipboard.writeText(generatedPassword); toast.success(t("common.copied")) }}>
              <Copy className="h-4 w-4" />
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={() => setGeneratedPassword("")}>{t("common.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
