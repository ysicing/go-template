import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router-dom"
import { toast } from "sonner"
import { Coins, ChevronRight } from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Card, CardContent } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { useAppStore } from "@/stores/app"
import { pointsApi } from "@/api/services"
import ProfileTab from "@/pages/profile/profile-tab"
import { AuthorizedAppsTab } from "@/pages/profile/authorized-apps-tab"
import { SocialAccountsTab } from "@/pages/profile/social-accounts-tab"
import SecurityPage from "@/pages/security"
import SessionsPage from "@/pages/sessions"
import LoginHistoryPage from "@/pages/login-history"

const COLORS = [
  "#ef4444", "#f97316", "#eab308", "#22c55e",
  "#06b6d4", "#3b82f6", "#6366f1", "#a855f7",
  "#ec4899",
]

export default function ProfilePage() {
  const { t, i18n } = useTranslation()
  const { primaryColor, setPrimaryColor, language, setLanguage } = useAppStore()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [pointsSummary, setPointsSummary] = useState<{ balance: number; level: number; streak: number } | null>(null)

  useEffect(() => {
    // Handle OAuth callback messages
    const success = searchParams.get("success")
    const error = searchParams.get("error")

    if (success === "account_linked") {
      toast.success(t("profile.unlinkSuccess").replace("解绑", "绑定"))
      // Clear URL params
      setSearchParams({})
    } else if (error) {
      const errorMessages: Record<string, string> = {
        user_not_found: "用户不存在",
        account_already_linked: "该社交账号已被其他用户绑定",
        failed_to_link: "绑定失败，请稍后重试",
      }
      toast.error(errorMessages[error] || error)
      // Clear URL params
      setSearchParams({})
    }

    Promise.all([pointsApi.getMyPoints(), pointsApi.getCheckInStatus()]).then(([pRes, sRes]) => {
      setPointsSummary({
        balance: pRes.data.total_balance ?? 0,
        level: pRes.data.points?.level ?? 1,
        streak: sRes.data.streak_days ?? 0,
      })
    }).catch(() => {})
  }, [searchParams, setSearchParams, t])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">{t("app.profile")}</h1>

      {/* Points summary */}
      {pointsSummary && (
        <Card
          className="cursor-pointer hover:bg-muted/50 transition-colors"
          onClick={() => navigate("/account/points")}
        >
          <CardContent className="flex items-center justify-between py-4">
            <div className="flex items-center gap-4">
              <Coins className="h-5 w-5 text-primary" />
              <div className="flex gap-6 text-sm">
                <span>{t("points.totalBalance")}: <span className="font-semibold">{pointsSummary.balance}</span></span>
                <span>{t("points.level")}: <span className="font-semibold">Lv.{pointsSummary.level}</span></span>
                <span>{t("points.streakDays", { days: pointsSummary.streak })}</span>
              </div>
            </div>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="profile" className="w-full">
        <TabsList>
          <TabsTrigger value="profile">{t("app.profileTabProfile")}</TabsTrigger>
          <TabsTrigger value="social-accounts">{t("profile.socialAccounts")}</TabsTrigger>
          <TabsTrigger value="authorized-apps">{t("profile.authorizedApps")}</TabsTrigger>
          <TabsTrigger value="security">{t("app.profileTabSecurity")}</TabsTrigger>
          <TabsTrigger value="sessions">{t("app.profileTabSessions")}</TabsTrigger>
          <TabsTrigger value="login-history">{t("app.profileTabLoginHistory")}</TabsTrigger>
          <TabsTrigger value="appearance">{t("app.profileTabAppearance")}</TabsTrigger>
        </TabsList>

        <TabsContent value="profile">
          <ProfileTab />
        </TabsContent>

        <TabsContent value="social-accounts">
          <SocialAccountsTab />
        </TabsContent>

        <TabsContent value="authorized-apps">
          <AuthorizedAppsTab />
        </TabsContent>

        <TabsContent value="security">
          <SecurityPage hideTitle />
        </TabsContent>

        <TabsContent value="sessions">
          <SessionsPage hideTitle />
        </TabsContent>

        <TabsContent value="login-history">
          <LoginHistoryPage hideTitle />
        </TabsContent>

        <TabsContent value="appearance">
          <Card>
            <CardContent className="space-y-6 pt-6">
              <div className="space-y-3">
                <Label>{t("settings.themeColor")}</Label>
                <div className="flex flex-wrap gap-3">
                  {COLORS.map((c) => (
                    <button
                      key={c}
                      onClick={() => setPrimaryColor(c)}
                      className="h-8 w-8 rounded-full border-2 transition-transform hover:scale-110"
                      style={{
                        backgroundColor: c,
                        borderColor: primaryColor === c ? c : "transparent",
                        boxShadow: primaryColor === c ? `0 0 0 2px var(--background), 0 0 0 4px ${c}` : undefined,
                      }}
                    />
                  ))}
                </div>
              </div>
              <Separator />
              <div className="space-y-3">
                <Label>{t("settings.language")}</Label>
                <div className="flex gap-2">
                  {[
                    { value: "zh", label: "中文" },
                    { value: "en", label: "English" },
                  ].map((l) => (
                    <Button
                      key={l.value}
                      variant={language === l.value ? "default" : "outline"}
                      size="sm"
                      onClick={() => {
                        setLanguage(l.value as "en" | "zh")
                        i18n.changeLanguage(l.value)
                      }}
                    >
                      {l.label}
                    </Button>
                  ))}
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
