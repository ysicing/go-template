import { useEffect, useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Mail, Loader2, CheckCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { authApi } from "@/api/services"

export default function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get("token")
  const [loading, setLoading] = useState(false)
  const [verified, setVerified] = useState(false)
  const [resending, setResending] = useState(false)
  const navigate = useNavigate()
  const { t } = useTranslation()

  useEffect(() => {
    if (!token) return
    setLoading(true)
    authApi.verifyEmail(token)
      .then(() => {
        setVerified(true)
        toast.success(t("email.verified"))
      })
      .catch((err: unknown) => {
        const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || t("email.invalidToken")
        toast.error(msg)
      })
      .finally(() => setLoading(false))
  }, [token, t])

  const handleResend = async () => {
    setResending(true)
    try {
      await authApi.resendVerification()
      toast.success(t("email.resent"))
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || t("common.error")
      toast.error(msg)
    } finally {
      setResending(false)
    }
  }

  if (verified) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <Card className="w-full max-w-sm">
          <CardHeader className="space-y-1 text-center">
            <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-green-100 text-green-600">
              <CheckCircle className="h-6 w-6" />
            </div>
            <h1 className="text-xl font-semibold">{t("email.verifiedTitle")}</h1>
            <p className="text-sm text-muted-foreground">{t("email.verifiedDesc")}</p>
          </CardHeader>
          <CardContent>
            <Button className="w-full" onClick={() => navigate("/")}>
              {t("email.goHome")}
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Mail className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold">{t("email.verifyTitle")}</h1>
          <p className="text-sm text-muted-foreground">
            {loading ? t("email.verifying") : t("email.checkInbox")}
          </p>
        </CardHeader>
        <CardContent className="space-y-4">
          {loading && (
            <div className="flex justify-center">
              <Loader2 className="h-6 w-6 animate-spin" />
            </div>
          )}
          {!loading && (
            <Button className="w-full" onClick={handleResend} disabled={resending}>
              {resending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("email.resend")}
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
