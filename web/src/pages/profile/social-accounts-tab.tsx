import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Github } from "lucide-react"
import { userApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"

interface SocialAccount {
  id: string
  provider: string
  provider_id: string
  username: string
  display_name: string
  email: string
  avatar_url: string
  created_at: string
}

function GoogleIcon() {
  return (
    <svg className="h-5 w-5" viewBox="0 0 24 24">
      <path fill="currentColor" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
      <path fill="currentColor" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
      <path fill="currentColor" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
      <path fill="currentColor" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
    </svg>
  )
}

export function SocialAccountsTab() {
  const { t } = useTranslation()
  const [accounts, setAccounts] = useState<SocialAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [unlinkingId, setUnlinkingId] = useState<string | null>(null)

  const loadAccounts = useCallback(async () => {
    try {
      setErrorKind(null)
      const response = await userApi.listSocialAccounts()
      setAccounts(response.data || [])
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadAccounts()
  }, [loadAccounts])

  const handleUnlink = async (id: string) => {
    try {
      await userApi.unlinkSocialAccount(id)
      toast.success(t("profile.unlinkSuccess"))
      setAccounts((current) => current.filter((account) => account.id !== id))
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setUnlinkingId(null)
    }
  }

  const getProviderIcon = (provider: string) => {
    switch (provider.toLowerCase()) {
      case "github":
        return <Github className="h-5 w-5" />
      case "google":
        return <GoogleIcon />
      default:
        return null
    }
  }

  const getProviderName = (provider: string) => t(`profile.${provider.toLowerCase()}`, provider)

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.socialAccounts")}</CardTitle>
          <CardDescription>{t("profile.socialAccountsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="py-8 text-center text-muted-foreground">{t("common.loading")}</div>
        </CardContent>
      </Card>
    )
  }

  if (errorKind) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.socialAccounts")}</CardTitle>
          <CardDescription>{t("profile.socialAccountsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <PageErrorState kind={errorKind} onRetry={() => void loadAccounts()} compact />
        </CardContent>
      </Card>
    )
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.socialAccounts")}</CardTitle>
          <CardDescription>{t("profile.socialAccountsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {accounts.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              {t("profile.noSocialAccounts")}
            </div>
          ) : (
            <div className="space-y-4">
              {accounts.map((account) => (
                <div key={account.id} className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex items-center gap-4">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                      {getProviderIcon(account.provider)}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{getProviderName(account.provider)}</span>
                        {account.email ? (
                          <Badge variant="secondary" className="text-xs">
                            {account.email}
                          </Badge>
                        ) : null}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {t("profile.createdAt")}: {new Date(account.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                  <Button variant="outline" size="sm" onClick={() => setUnlinkingId(account.id)}>
                    {t("profile.unlinkAccount")}
                  </Button>
                </div>
              ))}
            </div>
          )}

          <div className="mt-6 border-t pt-6">
            <h3 className="mb-3 text-sm font-medium">{t("profile.bindAccount")}</h3>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => { window.location.href = "/api/auth/github" }}
                className="flex items-center gap-2"
              >
                <Github className="h-4 w-4" />
                GitHub
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => { window.location.href = "/api/auth/google" }}
                className="flex items-center gap-2"
              >
                <GoogleIcon />
                Google
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={!!unlinkingId} onOpenChange={() => setUnlinkingId(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("profile.unlinkAccount")}</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p className="text-sm text-muted-foreground">{t("profile.unlinkConfirm")}</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUnlinkingId(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => unlinkingId && handleUnlink(unlinkingId)}>
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
