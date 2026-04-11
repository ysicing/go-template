import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ShieldCheck, Key, Copy, Trash2, Plus, Download, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { mfaApi, webauthnApi } from "@/api/services"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import PageErrorState from "@/components/page-error-state"

interface Credential {
  id: string
  name: string
  created_at: string
}

export default function SecurityPage({ hideTitle }: { hideTitle?: boolean }) {
  const { t } = useTranslation()
  const [totpEnabled, setTotpEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)

  // TOTP setup state
  const [setupSecret, setSetupSecret] = useState("")
  const [setupUrl, setSetupUrl] = useState("")
  const [confirmCode, setConfirmCode] = useState("")
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [showSetup, setShowSetup] = useState(false)

  // Disable state
  const [disablePassword, setDisablePassword] = useState("")
  const [showDisable, setShowDisable] = useState(false)

  // WebAuthn state
  const [credentials, setCredentials] = useState<Credential[]>([])

  const loadCredentials = async () => {
    try {
      setErrorKind(null)
      const res = await webauthnApi.listCredentials()
      setCredentials(res.data.credentials || [])
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
    }
  }

  useEffect(() => {
    const init = async () => {
      try {
        setErrorKind(null)
        const [statusRes, credRes] = await Promise.all([
          mfaApi.status(),
          webauthnApi.listCredentials(),
        ])
        setTotpEnabled(statusRes.data.totp_enabled)
        setCredentials(credRes.data.credentials || [])
      } catch (error) {
        setErrorKind(getApiErrorKind(error))
      }
      setLoading(false)
    }
    void init()
  }, [])

  const handleSetup = async () => {
    try {
      const res = await mfaApi.totpSetup()
      setSetupSecret(res.data.secret)
      setSetupUrl(res.data.url)
      setShowSetup(true)
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    }
  }

  const handleEnable = async () => {
    if (!confirmCode) return
    try {
      const res = await mfaApi.totpEnable(confirmCode)
      setTotpEnabled(true)
      setBackupCodes(res.data.backup_codes || [])
      setShowSetup(false)
      setConfirmCode("")
      toast.success(t("mfa.enabled"))
    } catch (error) {
      toast.error(getErrorMessage(error, t("mfa.invalidCode")))
    }
  }

  const handleDisable = async () => {
    if (!disablePassword) return
    try {
      await mfaApi.totpDisable(disablePassword)
      setTotpEnabled(false)
      setShowDisable(false)
      setDisablePassword("")
      toast.success(t("mfa.disabled"))
    } catch (error) {
      toast.error(getErrorMessage(error, t("login.error")))
    }
  }

  const handleRegenerateBackup = async () => {
    try {
      const res = await mfaApi.regenerateBackupCodes()
      setBackupCodes(res.data.backup_codes || [])
      toast.success(t("mfa.regenerated"))
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    }
  }

  const handleRegisterPasskey = async () => {
    try {
      const beginRes = await webauthnApi.registerBegin()
      const options = beginRes.data

      // Convert base64url to ArrayBuffer for WebAuthn API
      options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge)
      options.publicKey.user.id = base64urlToBuffer(options.publicKey.user.id)
      if (options.publicKey.excludeCredentials) {
        options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map(
          (c: { id: string; type: string }) => ({ ...c, id: base64urlToBuffer(c.id) })
        )
      }

      const credential = await navigator.credentials.create(options) as PublicKeyCredential
      if (!credential) return

      const response = credential.response as AuthenticatorAttestationResponse
      const body = {
        id: credential.id,
        rawId: bufferToBase64url(credential.rawId),
        type: credential.type,
        response: {
          attestationObject: bufferToBase64url(response.attestationObject),
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
        },
      }

      const name = prompt(t("webauthn.name")) || "Passkey"
      await webauthnApi.registerFinish(body, name)
      toast.success(t("webauthn.registered"))
      void loadCredentials()
    } catch (err) {
      const msg = getErrorMessage(err, "")
      if (msg === "webauthn not configured") {
        toast.error(t("webauthn.notConfigured"))
        return
      }
      toast.error(msg || t("common.error"))
    }
  }

  const handleDeleteCredential = async (id: string) => {
    if (!confirm(t("webauthn.deleteConfirm"))) return
    try {
      await webauthnApi.deleteCredential(id)
      toast.success(t("webauthn.deleted"))
      void loadCredentials()
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    }
  }

  if (loading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} compact={hideTitle} />
  }

  return (
    <div className="space-y-6">
      {!hideTitle && <h1 className="text-2xl font-semibold">{t("app.security")}</h1>}

      {/* TOTP Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-5 w-5" />
              <CardTitle className="text-lg">{t("mfa.totp")}</CardTitle>
            </div>
            <Badge variant={totpEnabled ? "default" : "secondary"}>
              {totpEnabled ? t("mfa.totpEnabled") : t("mfa.totpDisabled")}
            </Badge>
          </div>
          <CardDescription>{t("mfa.totpDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!totpEnabled && !showSetup && (
            <Button onClick={handleSetup}>{t("mfa.enable")}</Button>
          )}

          {showSetup && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">{t("mfa.setupDesc")}</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono">{setupSecret}</code>
                <Button variant="ghost" size="icon" onClick={() => { navigator.clipboard.writeText(setupSecret); toast.success(t("common.copied")) }}>
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
              {setupUrl && (
                <img
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(setupUrl)}`}
                  alt="TOTP QR Code"
                  className="mx-auto h-48 w-48 rounded border"
                />
              )}
              <div className="space-y-2">
                <Label>{t("mfa.confirmCode")}</Label>
                <div className="flex gap-2">
                  <Input value={confirmCode} onChange={(e) => setConfirmCode(e.target.value)} placeholder="000000" />
                  <Button onClick={handleEnable} disabled={!confirmCode}>{t("mfa.enable")}</Button>
                </div>
              </div>
            </div>
          )}

          {totpEnabled && !showDisable && (
            <div className="flex gap-2">
              <Button variant="outline" onClick={handleRegenerateBackup}>{t("mfa.regenerateBackupCodes")}</Button>
              <Button variant="destructive" onClick={() => setShowDisable(true)}>{t("mfa.disable")}</Button>
            </div>
          )}

          {showDisable && (
            <div className="space-y-2">
              <Label>{t("mfa.disableConfirm")}</Label>
              <div className="flex gap-2">
                <Input type="password" value={disablePassword} onChange={(e) => setDisablePassword(e.target.value)} placeholder={t("mfa.password")} />
                <Button variant="destructive" onClick={handleDisable} disabled={!disablePassword}>{t("mfa.disable")}</Button>
                <Button variant="ghost" onClick={() => setShowDisable(false)}>{t("common.cancel")}</Button>
              </div>
            </div>
          )}

          {backupCodes.length > 0 && (
            <div className="space-y-2">
              <Separator />
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">{t("mfa.backupCodes")}</p>
                  <p className="text-xs text-muted-foreground">{t("mfa.backupCodesDesc")}</p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    const content = backupCodes.join("\n")
                    const blob = new Blob([content], { type: "text/plain" })
                    const url = URL.createObjectURL(blob)
                    const a = document.createElement("a")
                    a.href = url
                    a.download = "backup-codes.txt"
                    a.click()
                    URL.revokeObjectURL(url)
                  }}
                >
                  <Download className="mr-1 h-4 w-4" />
                  {t("mfa.downloadBackupCodes")}
                </Button>
              </div>
              <div className="grid grid-cols-2 gap-1">
                {backupCodes.map((code, i) => (
                  <code key={i} className="rounded bg-muted px-2 py-1 text-center text-sm font-mono">{code}</code>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* WebAuthn Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Key className="h-5 w-5" />
              <CardTitle className="text-lg">{t("webauthn.title")}</CardTitle>
            </div>
            <Button size="sm" onClick={handleRegisterPasskey}>
              <Plus className="mr-1 h-4 w-4" /> {t("webauthn.register")}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {credentials.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("webauthn.noCredentials")}</p>
          ) : (
            <div className="space-y-2">
              {credentials.map((cred) => (
                <div key={cred.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div>
                    <p className="text-sm font-medium">{cred.name}</p>
                    <p className="text-xs text-muted-foreground">{new Date(cred.created_at).toLocaleDateString()}</p>
                  </div>
                  <Button variant="ghost" size="icon" onClick={() => handleDeleteCredential(cred.id)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/")
  const pad = base64.length % 4 === 0 ? "" : "=".repeat(4 - (base64.length % 4))
  const binary = atob(base64 + pad)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ""
  for (const b of bytes) binary += String.fromCharCode(b)
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}
