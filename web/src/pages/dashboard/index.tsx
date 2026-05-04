import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowRight,
  Coins,
  KeyRound,
  Loader2,
  ShieldCheck,
  UserCircle,
  Users,
} from "lucide-react"
import { statsApi } from "@/api/services"
import { getApiErrorKind, type ApiErrorKind } from "@/api/client"
import { adminPermissions, hasAnyAdminPermission, hasPermission } from "@/lib/permissions"
import { getConsoleModuleEntry } from "@/lib/navigation"
import { useAuthStore } from "@/stores/auth"
import PageErrorState from "@/components/page-error-state"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

interface QuickAction {
  title: string
  description: string
  href: string
  icon: typeof UserCircle
}

export default function DashboardPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const canReadAdminStats = hasPermission(user, adminPermissions.statsRead)
  const canAccessAdmin = hasAnyAdminPermission(user)
  const adminEntry = getConsoleModuleEntry("admin", user)
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [adminStats, setAdminStats] = useState({
    total_users: 0,
    total_logins: 0,
    today_logins: 0,
  })

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      setErrorKind(null)

      try {
        const adminRes = canReadAdminStats ? await statsApi.admin() : null

        if (adminRes) {
          setAdminStats({
            total_users: adminRes.data.total_users ?? 0,
            total_logins: adminRes.data.total_logins ?? 0,
            today_logins: adminRes.data.today_logins ?? 0,
          })
        }
      } catch (error) {
        setErrorKind(getApiErrorKind(error))
      } finally {
        setLoading(false)
      }
    }

    void fetchData()
  }, [canReadAdminStats])

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => window.location.reload()} />
  }

  const platformCards = [
    { title: t("dashboard.platformUsers"), value: adminStats.total_users, icon: Users },
    { title: t("dashboard.platformLogins"), value: adminStats.total_logins, icon: ShieldCheck },
    { title: t("dashboard.platformToday"), value: adminStats.today_logins, icon: ArrowRight },
  ]

  const quickActions: QuickAction[] = [
    {
      title: t("app.profile"),
      description: t("dashboard.quickAccessProfile"),
      href: "/profile",
      icon: UserCircle,
    },
    {
      title: t("points.title"),
      description: t("dashboard.quickAccessPoints"),
      href: "/points",
      icon: Coins,
    },
  ]

  if (canAccessAdmin && adminEntry) {
    quickActions.push({
      title: t("app.admin"),
      description: t("dashboard.quickAccessAdmin"),
      href: adminEntry,
      icon: KeyRound,
    })
  }

  const secondaryAction = canAccessAdmin && adminEntry
    ? { href: adminEntry, label: t("app.admin") }
    : { href: "/points", label: t("points.title") }

  return (
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-3xl border bg-linear-to-br from-slate-950 via-slate-900 to-sky-900 px-6 py-8 text-white shadow-sm">
        <div className="absolute inset-y-0 right-0 hidden w-1/2 bg-[radial-gradient(circle_at_top,_rgba(125,211,252,0.26),_transparent_58%)] lg:block" />
        <div className="relative grid gap-6 lg:grid-cols-[1.55fr_0.95fr]">
          <div className="space-y-4">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-xs uppercase tracking-[0.2em] text-sky-100">
              <ShieldCheck className="h-3.5 w-3.5" />
              {t("dashboard.badge")}
            </div>
            <div className="space-y-2">
              <h1 className="max-w-2xl text-3xl font-semibold tracking-tight sm:text-4xl">
                {t("dashboard.heroTitle")}
              </h1>
              <p className="max-w-2xl text-sm leading-6 text-slate-200 sm:text-base">
                {t("dashboard.heroDescription")}
              </p>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <Button asChild size="lg" className="bg-white text-slate-950 hover:bg-slate-100">
                <Link to="/profile">{t("app.profile")}</Link>
              </Button>
              <Button
                asChild
                size="lg"
                variant="outline"
                className="border-white/20 bg-white/5 text-white hover:bg-white/10 hover:text-white"
              >
                <Link to={secondaryAction.href}>
                  {secondaryAction.label}
                  <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
            <div className="rounded-2xl border border-dashed border-white/20 bg-slate-950/20 p-4 text-sm text-slate-200 sm:col-span-2 lg:col-span-1">
              <p className="font-medium text-white">{t("dashboard.quickAccessTitle")}</p>
              <p className="mt-1 text-slate-300">{t("common.total", { count: quickActions.length })}</p>
            </div>
          </div>
        </div>
      </section>

      <Card className="rounded-3xl">
        <CardHeader>
          <CardTitle>{t("dashboard.quickAccessTitle")}</CardTitle>
          <CardDescription>{t("dashboard.quickAccessDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {quickActions.map((action) => (
            <Link
              key={action.href}
              to={action.href}
              className="rounded-2xl border bg-background p-4 transition-colors hover:bg-muted/30"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <p className="font-medium">{action.title}</p>
                  <p className="text-sm text-muted-foreground">{action.description}</p>
                </div>
                <action.icon className="mt-0.5 h-4 w-4 text-muted-foreground" />
              </div>
            </Link>
          ))}
        </CardContent>
      </Card>

      {canReadAdminStats && (
        <Card className="rounded-3xl">
          <CardHeader>
            <CardTitle>{t("dashboard.platformSnapshotTitle")}</CardTitle>
            <CardDescription>{t("dashboard.platformSnapshotDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            {platformCards.map((card) => (
              <div key={card.title} className="rounded-2xl border bg-background p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm text-muted-foreground">{card.title}</p>
                  <card.icon className="h-4 w-4 text-muted-foreground" />
                </div>
                <p className="mt-3 text-3xl font-semibold">{card.value}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
