import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { userApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"
import { GitHubIcon, GoogleIcon } from "@/components/icons"

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
        return <GitHubIcon className="h-5 w-5" />
      case "google":
        return <GoogleIcon className="h-5 w-5" />
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
                <GitHubIcon className="h-4 w-4" />
                GitHub
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => { window.location.href = "/api/auth/google" }}
                className="flex items-center gap-2"
              >
                <GoogleIcon className="h-4 w-4" />
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
