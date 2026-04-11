import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Copy, Check, Loader2 } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuthStore } from "@/stores/auth"
import { userApi, authApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { toast } from "sonner"
import PageErrorState from "@/components/page-error-state"

export default function ProfileTab() {
  const { t } = useTranslation()
  const { user, updateUser } = useAuthStore()
  const [loading, setLoading] = useState(false)
  const [bootLoading, setBootLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [copied, setCopied] = useState(false)

  // Email dialog
  const [emailDialogOpen, setEmailDialogOpen] = useState(false)
  const [newEmail, setNewEmail] = useState("")

  // Password dialogs
  const [changePasswordOpen, setChangePasswordOpen] = useState(false)
  const [setPasswordOpen, setSetPasswordOpen] = useState(false)
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")

  // Resend verification cooldown
  const [resendCooldown, setResendCooldown] = useState(0)

  useEffect(() => {
    // Load latest user info
    setBootLoading(true)
    userApi.getMe().then(res => {
      setErrorKind(null)
      if (res.data.user) {
        updateUser(res.data.user)
      }
    }).catch((error) => {
      setErrorKind(getApiErrorKind(error))
    }).finally(() => {
      setBootLoading(false)
    })
  }, [updateUser])

  useEffect(() => {
    if (resendCooldown > 0) {
      const timer = setTimeout(() => setResendCooldown(resendCooldown - 1), 1000)
      return () => clearTimeout(timer)
    }
  }, [resendCooldown])

  const handleCopyId = () => {
    if (user?.id) {
      navigator.clipboard.writeText(user.id)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const handleUpdateEmail = async () => {
    if (!newEmail.trim()) return
    setLoading(true)
    try {
      const res = await userApi.updateMe({ email: newEmail.trim() })
      if (res.data.user) {
        updateUser(res.data.user)
      }
      toast.success(t("profile.emailUpdated"))
      setEmailDialogOpen(false)
      setNewEmail("")
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  const handleChangePassword = async () => {
    if (!currentPassword || !newPassword) return
    setLoading(true)
    try {
      await userApi.changePassword(currentPassword, newPassword)
      toast.success(t("profile.passwordChanged"))
      setChangePasswordOpen(false)
      setCurrentPassword("")
      setNewPassword("")
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  const handleSetPassword = async () => {
    if (!newPassword) return
    setLoading(true)
    try {
      await userApi.setPassword(newPassword)
      toast.success(t("profile.passwordSet"))
      setSetPasswordOpen(false)
      setNewPassword("")
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  const handleResendVerification = async () => {
    setLoading(true)
    try {
      await authApi.resendVerification()
      toast.success(t("profile.resendSuccess"))
      setResendCooldown(120) // 2 minutes
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  if (bootLoading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} compact />
  }

  if (!user) return null

  const providerLabel = user.provider === "local" ? t("profile.local")
    : user.provider === "github" ? t("profile.github")
    : user.provider === "google" ? t("profile.google")
    : user.provider

  const hasPassword = user.provider === "local" || (user.provider !== "local" && user.email_verified)

  return (
    <div className="space-y-6">
      {/* Basic Info */}
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.basicInfo")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label>{t("profile.username")}</Label>
            <div className="text-sm">{user.username}</div>
          </div>
          <div className="grid gap-2">
            <Label>{t("profile.userId")}</Label>
            <div className="flex items-center gap-2">
              <code className="text-xs bg-muted px-2 py-1 rounded">{user.id}</code>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCopyId}
                className="h-7 w-7 p-0"
              >
                {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
              </Button>
            </div>
          </div>
          <div className="grid gap-2">
            <Label>{t("profile.provider")}</Label>
            <div className="text-sm">{providerLabel}</div>
          </div>
          {user.created_at && (
            <div className="grid gap-2">
              <Label>{t("profile.createdAt")}</Label>
              <div className="text-sm">{new Date(user.created_at).toLocaleString()}</div>
            </div>
          )}
        </CardContent>
      </Card>

      <Separator />

      {/* Email Settings */}
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.emailSettings")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label>{t("profile.email")}</Label>
            <div className="flex items-center gap-2">
              <span className="text-sm">{user.email}</span>
              <Badge variant={user.email_verified ? "default" : "secondary"}>
                {user.email_verified ? t("profile.emailVerified") : t("profile.emailNotVerified")}
              </Badge>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => setEmailDialogOpen(true)}>
              {t("profile.editEmail")}
            </Button>
            {!user.email_verified && (
              <Button
                variant="outline"
                size="sm"
                onClick={handleResendVerification}
                disabled={loading || resendCooldown > 0}
              >
                {resendCooldown > 0
                  ? `${t("profile.resendVerification")} (${resendCooldown}s)`
                  : t("profile.resendVerification")}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Separator />

      {/* Password Settings */}
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.passwordSettings")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {hasPassword ? (
            <Button variant="outline" size="sm" onClick={() => setChangePasswordOpen(true)}>
              {t("profile.changePassword")}
            </Button>
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">{t("profile.setPasswordDesc")}</p>
              <Button variant="outline" size="sm" onClick={() => setSetPasswordOpen(true)}>
                {t("profile.setPassword")}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Email Dialog */}
      <Dialog open={emailDialogOpen} onOpenChange={setEmailDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("profile.editEmail")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="new-email">{t("profile.newEmail")}</Label>
              <Input
                id="new-email"
                type="email"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                placeholder={user.email}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEmailDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleUpdateEmail} disabled={loading || !newEmail.trim()}>
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Change Password Dialog */}
      <Dialog open={changePasswordOpen} onOpenChange={setChangePasswordOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("profile.changePassword")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="current-password">{t("profile.currentPassword")}</Label>
              <Input
                id="current-password"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">{t("profile.newPassword")}</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">{t("profile.passwordHint")}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setChangePasswordOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleChangePassword} disabled={loading || !currentPassword || !newPassword}>
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Set Password Dialog */}
      <Dialog open={setPasswordOpen} onOpenChange={setSetPasswordOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("profile.setPassword")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="set-password">{t("profile.newPassword")}</Label>
              <Input
                id="set-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">{t("profile.passwordHint")}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSetPasswordOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSetPassword} disabled={loading || !newPassword}>
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
