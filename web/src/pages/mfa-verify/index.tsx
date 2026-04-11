import { useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ShieldCheck, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { authApi } from "@/api/services"
import { useAuthStore } from "@/stores/auth"
import { redirectToSameOrigin } from "@/lib/navigation"

export default function MFAVerifyPage() {
  const [code, setCode] = useState("")
  const [useBackup, setUseBackup] = useState(false)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()
  const { setUser } = useAuthStore()

  const mfaToken = searchParams.get("mfa_token") || ""

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!mfaToken || !code) return
    setLoading(true)
    try {
      const res = await authApi.mfaVerify(
        mfaToken,
        useBackup ? undefined : code,
        useBackup ? code : undefined,
      )
      if (res.data.redirect) {
        if (!redirectToSameOrigin(res.data.redirect as string)) {
          navigate("/")
        }
        return
      }
      const { user } = res.data
      // Tokens are set in HttpOnly cookies
      setUser(user)
      navigate("/")
    } catch {
      toast.error(t("mfa.invalidCode"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold">{t("mfa.verify")}</h1>
          <p className="text-sm text-muted-foreground">{t("mfa.verifyDesc")}</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="code">
                {useBackup ? t("mfa.backupCode") : t("mfa.totpCode")}
              </Label>
              <Input
                id="code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
                autoFocus
                placeholder={useBackup ? "xxxxxxxx" : "000000"}
                autoComplete="one-time-code"
              />
            </div>
            <Button type="submit" className="w-full" disabled={loading || !code}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("mfa.submit")}
            </Button>
            <button
              type="button"
              onClick={() => { setUseBackup(!useBackup); setCode("") }}
              className="w-full text-center text-sm text-muted-foreground hover:text-primary"
            >
              {useBackup ? t("mfa.useTotpCode") : t("mfa.useBackupCode")}
            </button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
