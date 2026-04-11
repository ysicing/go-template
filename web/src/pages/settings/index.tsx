import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Loader2, Send } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { adminSettingsApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { useHasPermission, adminPermissions } from "@/lib/permissions"
import PageErrorState from "@/components/page-error-state"

export default function SettingsPage() {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.settingsRead)
  const canWrite = useHasPermission(adminPermissions.settingsWrite)
  const [loading, setLoading] = useState(false)
  const [bootLoading, setBootLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [registerEnabled, setRegisterEnabled] = useState(true)
  const [siteTitle, setSiteTitle] = useState("")
  const [corsOrigins, setCorsOrigins] = useState("")
  const [webauthnRpId, setWebauthnRpId] = useState("")
  const [webauthnRpDisplayName, setWebauthnRpDisplayName] = useState("")
  const [webauthnRpOrigins, setWebauthnRpOrigins] = useState("")
  const [turnstileSiteKey, setTurnstileSiteKey] = useState("")
  const [turnstileSecretKey, setTurnstileSecretKey] = useState("")
  const [smtpHost, setSmtpHost] = useState("")
  const [smtpPort, setSmtpPort] = useState("587")
  const [smtpUsername, setSmtpUsername] = useState("")
  const [smtpPassword, setSmtpPassword] = useState("")
  const [smtpFromAddress, setSmtpFromAddress] = useState("")
  const [smtpTls, setSmtpTls] = useState(true)
  const [emailVerificationEnabled, setEmailVerificationEnabled] = useState(false)
  const [emailDomainMode, setEmailDomainMode] = useState("disabled")
  const [emailDomainWhitelist, setEmailDomainWhitelist] = useState("")
  const [emailDomainBlacklist, setEmailDomainBlacklist] = useState("")
  const [testEmailTo, setTestEmailTo] = useState("")
  const [testEmailLoading, setTestEmailLoading] = useState(false)

  useEffect(() => {
    if (!canRead) return
    setBootLoading(true)
    setErrorKind(null)
    adminSettingsApi.get().then((res) => {
      setRegisterEnabled(res.data.register_enabled ?? true)
      setSiteTitle(res.data.site_title ?? "")
      setCorsOrigins(res.data.cors_origins ?? "")
      setWebauthnRpId(res.data.webauthn_rp_id ?? "")
      setWebauthnRpDisplayName(res.data.webauthn_rp_display_name ?? "")
      setWebauthnRpOrigins(res.data.webauthn_rp_origins ?? "")
      setTurnstileSiteKey(res.data.turnstile_site_key ?? "")
      setTurnstileSecretKey(res.data.turnstile_secret_key ?? "")
      setSmtpHost(res.data.smtp_host ?? "")
      setSmtpPort(res.data.smtp_port ?? "587")
      setSmtpUsername(res.data.smtp_username ?? "")
      setSmtpPassword(res.data.smtp_password ?? "")
      setSmtpFromAddress(res.data.smtp_from_address ?? "")
      setSmtpTls(res.data.smtp_tls ?? true)
      setEmailVerificationEnabled(res.data.email_verification_enabled ?? false)
      setEmailDomainMode(res.data.email_domain_mode ?? "disabled")
      setEmailDomainWhitelist(res.data.email_domain_whitelist ?? "")
      setEmailDomainBlacklist(res.data.email_domain_blacklist ?? "")
    }).catch((error) => {
      setErrorKind(getApiErrorKind(error))
    }).finally(() => {
      setBootLoading(false)
    })
  }, [canRead])

  const handleSave = async () => {
    if (!canWrite) return
    setLoading(true)
    try {
      const payload: Record<string, unknown> = {
        register_enabled: registerEnabled,
        site_title: siteTitle,
        cors_origins: corsOrigins,
        webauthn_rp_id: webauthnRpId,
        webauthn_rp_display_name: webauthnRpDisplayName,
        webauthn_rp_origins: webauthnRpOrigins,
        smtp_host: smtpHost,
        smtp_port: smtpPort,
        smtp_username: smtpUsername,
        smtp_from_address: smtpFromAddress,
        smtp_tls: smtpTls,
        email_verification_enabled: emailVerificationEnabled,
        email_domain_mode: emailDomainMode,
        email_domain_whitelist: emailDomainWhitelist,
        email_domain_blacklist: emailDomainBlacklist,
      }
      if (turnstileSiteKey) {
        payload.turnstile_site_key = turnstileSiteKey
      }
      if (turnstileSecretKey && !turnstileSecretKey.includes("***")) {
        payload.turnstile_secret_key = turnstileSecretKey
      }
      if (!smtpPassword.includes("***")) {
        payload.smtp_password = smtpPassword
      }
      await adminSettingsApi.update(payload)
      toast.success(t("settings.saved"))
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setLoading(false)
    }
  }

  const handleTestEmail = async () => {
    if (!canWrite) return
    setTestEmailLoading(true)
    try {
      await adminSettingsApi.testEmail(testEmailTo.trim())
      toast.success(t("settings.testEmailSent"))
    } catch (err) {
      toast.error(getErrorMessage(err, t("common.error")))
    } finally {
      setTestEmailLoading(false)
    }
  }

  const SaveButton = () => (
    <Button onClick={handleSave} disabled={loading || !canWrite} className="mt-2">
      {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
      {t("settings.save")}
    </Button>
  )

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  if (bootLoading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} />
  }

  return (
    <div className="max-w-2xl space-y-6">
      <h1 className="text-2xl font-semibold">{t("settings.title")}</h1>

      <Tabs defaultValue="general" className="w-full">
        <TabsList className="grid w-full grid-cols-3">
          <TabsTrigger value="general">{t("settings.tabGeneral")}</TabsTrigger>
          <TabsTrigger value="email">{t("settings.tabEmail")}</TabsTrigger>
          <TabsTrigger value="webauthn">{t("settings.tabWebauthn")}</TabsTrigger>
        </TabsList>

        <TabsContent value="general">
          <Card>
            <CardContent className="space-y-6 pt-6">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>{t("settings.registerEnabled")}</Label>
                  <p className="text-sm text-muted-foreground">{t("settings.registerEnabledDesc")}</p>
                </div>
                <Switch checked={registerEnabled} onCheckedChange={setRegisterEnabled} />
              </div>
              <Separator />
              <div className="space-y-2">
                <Label>{t("settings.siteTitle")}</Label>
                <Input
                  value={siteTitle}
                  onChange={(e) => setSiteTitle(e.target.value)}
                  placeholder={t("settings.siteTitlePlaceholder")}
                />
                <p className="text-sm text-muted-foreground">{t("settings.siteTitleDesc")}</p>
              </div>
              <Separator />
              <div className="space-y-2">
                <Label>{t("settings.corsOrigins")}</Label>
                <Input
                  value={corsOrigins}
                  onChange={(e) => setCorsOrigins(e.target.value)}
                  placeholder={t("settings.corsOriginsPlaceholder")}
                />
                <p className="text-sm text-muted-foreground">{t("settings.corsOriginsDesc")}</p>
              </div>
              <Separator />
              <div className="space-y-2">
                <Label className="text-sm font-medium">{t("settings.turnstile")}</Label>
                <p className="text-sm text-muted-foreground">{t("settings.turnstileDesc")}</p>
              </div>
              <div className="space-y-2">
                <Label>{t("settings.turnstileSiteKey")}</Label>
                <Input
                  value={turnstileSiteKey}
                  onChange={(e) => setTurnstileSiteKey(e.target.value)}
                  placeholder={t("settings.turnstileSiteKeyPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.turnstileSecretKey")}</Label>
                <Input
                  type="password"
                  value={turnstileSecretKey}
                  onChange={(e) => setTurnstileSecretKey(e.target.value)}
                  placeholder={t("settings.turnstileSecretKeyPlaceholder")}
                />
                <p className="text-sm text-muted-foreground">{t("settings.turnstileSecretKeyHint")}</p>
              </div>
              <SaveButton />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="email">
          <Card>
            <CardContent className="space-y-6 pt-6">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>{t("settings.emailVerification")}</Label>
                  <p className="text-sm text-muted-foreground">{t("settings.emailVerificationDesc")}</p>
                </div>
                <Switch checked={emailVerificationEnabled} onCheckedChange={setEmailVerificationEnabled} />
              </div>
              <Separator />
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>邮箱域名限制</Label>
                  <Select value={emailDomainMode} onValueChange={setEmailDomainMode}>
                    <SelectTrigger>
                      <SelectValue placeholder="选择模式" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="disabled">禁用</SelectItem>
                      <SelectItem value="whitelist">白名单模式</SelectItem>
                      <SelectItem value="blacklist">黑名单模式</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {emailDomainMode === "whitelist" && (
                  <div className="space-y-2">
                    <Label>白名单域名</Label>
                    <Input
                      value={emailDomainWhitelist}
                      onChange={(e) => setEmailDomainWhitelist(e.target.value)}
                      placeholder="company.com, partner.com"
                    />
                    <p className="text-xs text-muted-foreground">逗号分隔的域名</p>
                  </div>
                )}
                {emailDomainMode === "blacklist" && (
                  <div className="space-y-2">
                    <Label>黑名单域名</Label>
                    <Input
                      value={emailDomainBlacklist}
                      onChange={(e) => setEmailDomainBlacklist(e.target.value)}
                      placeholder="mailinator.com, tempmail.com"
                    />
                    <p className="text-xs text-muted-foreground">逗号分隔的域名</p>
                  </div>
                )}
              </div>
              <Separator />
              <div className="space-y-1">
                <Label className="text-sm font-medium">{t("settings.smtp")}</Label>
                <p className="text-sm text-muted-foreground">{t("settings.smtpDesc")}</p>
              </div>
              <div className="space-y-2">
                <Label>{t("settings.smtpHost")}</Label>
                <Input
                  value={smtpHost}
                  onChange={(e) => setSmtpHost(e.target.value)}
                  placeholder={t("settings.smtpHostPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.smtpPort")}</Label>
                <Input
                  value={smtpPort}
                  onChange={(e) => setSmtpPort(e.target.value)}
                  placeholder="587"
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.smtpUsername")}</Label>
                <Input
                  value={smtpUsername}
                  onChange={(e) => setSmtpUsername(e.target.value)}
                  placeholder={t("settings.smtpUsernamePlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.smtpPassword")}</Label>
                <Input
                  type="password"
                  value={smtpPassword}
                  onChange={(e) => setSmtpPassword(e.target.value)}
                  placeholder={t("settings.smtpPasswordPlaceholder")}
                />
                <p className="text-sm text-muted-foreground">{t("settings.smtpPasswordHint")}</p>
              </div>
              <div className="space-y-2">
                <Label>{t("settings.smtpFromAddress")}</Label>
                <Input
                  value={smtpFromAddress}
                  onChange={(e) => setSmtpFromAddress(e.target.value)}
                  placeholder={t("settings.smtpFromAddressPlaceholder")}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label>{t("settings.smtpTls")}</Label>
                  <p className="text-sm text-muted-foreground">{t("settings.smtpTlsDesc")}</p>
                </div>
                <Switch checked={smtpTls} onCheckedChange={setSmtpTls} />
              </div>
              <div className="space-y-3 rounded-md border p-4">
                <div className="space-y-1">
                  <Label>{t("settings.testEmailRecipient")}</Label>
                  <p className="text-sm text-muted-foreground">{t("settings.testEmailRecipientDesc")}</p>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Input
                    value={testEmailTo}
                    onChange={(e) => setTestEmailTo(e.target.value)}
                    placeholder={t("settings.testEmailRecipientPlaceholder")}
                  />
                  <Button variant="outline" onClick={handleTestEmail} disabled={testEmailLoading || !canWrite}>
                    {testEmailLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    {!testEmailLoading && <Send className="mr-2 h-4 w-4" />}
                    {t("settings.sendTestEmail")}
                  </Button>
                </div>
              </div>
              <SaveButton />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="webauthn">
          <Card>
            <CardContent className="space-y-6 pt-6">
              <div className="space-y-1">
                <Label className="text-sm font-medium">{t("settings.webauthn")}</Label>
                <p className="text-sm text-muted-foreground">{t("settings.webauthnDesc")}</p>
              </div>
              <Separator />
              <div className="space-y-2">
                <Label>{t("settings.webauthnRpId")}</Label>
                <Input
                  value={webauthnRpId}
                  onChange={(e) => setWebauthnRpId(e.target.value)}
                  placeholder={t("settings.webauthnRpIdPlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.webauthnRpDisplayName")}</Label>
                <Input
                  value={webauthnRpDisplayName}
                  onChange={(e) => setWebauthnRpDisplayName(e.target.value)}
                  placeholder={t("settings.webauthnRpDisplayNamePlaceholder")}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("settings.webauthnRpOrigins")}</Label>
                <Input
                  value={webauthnRpOrigins}
                  onChange={(e) => setWebauthnRpOrigins(e.target.value)}
                  placeholder={t("settings.webauthnRpOriginsPlaceholder")}
                />
                <p className="text-sm text-muted-foreground">{t("settings.webauthnRpOriginsDesc")}</p>
              </div>
              <SaveButton />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
