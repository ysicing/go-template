import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import {
  AppWindow,
  ArrowRight,
  CalendarCheck,
  CheckCircle2,
  Coins,
  KeyRound,
  Loader2,
  LogIn,
  Sparkles,
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
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface AppStat {
  client_id: string
  app_name: string
  login_count: number
}

interface SetupStep {
  title: string
  description: string
  href: string
  action: string
  complete: boolean
}

interface QuickAction {
  title: string
  description: string
  href: string
  icon: typeof AppWindow
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
    total_clients: 0,
    total_logins: 0,
    today_logins: 0,
  })
  const [userStats, setUserStats] = useState({
    my_login_count: 0,
    app_stats: [] as AppStat[],
  })

  useEffect(() => {
    const fetchStats = async () => {
      setLoading(true)
      setErrorKind(null)

      try {
        const userRes = await statsApi.user()
        setUserStats({
          my_login_count: userRes.data.my_login_count ?? 0,
          app_stats: userRes.data.app_stats ?? [],
        })

        if (canReadAdminStats) {
          const adminRes = await statsApi.admin()
          setAdminStats({
            total_users: adminRes.data.total_users ?? 0,
            total_clients: adminRes.data.total_clients ?? 0,
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

    void fetchStats()
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

  const appCount = userStats.app_stats.length
  const hasLogins = userStats.my_login_count > 0
  const completedSetupCount = [appCount > 0, appCount > 0, hasLogins].filter(Boolean).length
  const setupSteps: SetupStep[] = [
    {
      title: t("dashboard.setupCreateTitle"),
      description: t("dashboard.setupCreateDescription"),
      href: appCount > 0 ? "/uauth/apps" : "/uauth/apps/new",
      action: appCount > 0 ? t("dashboard.openApplications") : t("dashboard.startSetup"),
      complete: appCount > 0,
    },
    {
      title: t("dashboard.setupConfigureTitle"),
      description: t("dashboard.setupConfigureDescription"),
      href: "/uauth/apps",
      action: t("dashboard.reviewApplication"),
      complete: appCount > 0,
    },
    {
      title: t("dashboard.setupVerifyTitle"),
      description: t("dashboard.setupVerifyDescription"),
      href: appCount > 0 ? "/uauth/apps" : "/uauth/apps/new",
      action: hasLogins ? t("dashboard.viewApplications") : t("dashboard.runSignIn"),
      complete: hasLogins,
    },
  ]

  const personalCards = [
    { title: t("dashboard.personalApplications"), value: appCount, icon: AppWindow },
    { title: t("dashboard.personalLogins"), value: userStats.my_login_count, icon: LogIn },
  ]

  const platformCards = [
    { title: t("dashboard.platformUsers"), value: adminStats.total_users, icon: Users },
    { title: t("dashboard.platformApplications"), value: adminStats.total_clients, icon: KeyRound },
    { title: t("dashboard.platformLogins"), value: adminStats.total_logins, icon: LogIn },
    { title: t("dashboard.platformToday"), value: adminStats.today_logins, icon: CalendarCheck },
  ]

  const quickActions: QuickAction[] = [
    {
      title: t("app.apps"),
      description: t("dashboard.quickAccessApplications"),
      href: "/uauth/apps",
      icon: AppWindow,
    },
    {
      title: t("app.profile"),
      description: t("dashboard.quickAccessProfile"),
      href: "/account/profile",
      icon: UserCircle,
    },
    {
      title: t("points.title"),
      description: t("dashboard.quickAccessPoints"),
      href: "/account/points",
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

  return (
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-3xl border bg-linear-to-br from-slate-950 via-slate-900 to-sky-900 px-6 py-8 text-white shadow-sm">
        <div className="absolute inset-y-0 right-0 hidden w-1/2 bg-[radial-gradient(circle_at_top,_rgba(125,211,252,0.26),_transparent_58%)] lg:block" />
        <div className="relative grid gap-6 lg:grid-cols-[1.7fr_0.9fr]">
          <div className="space-y-4">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-xs uppercase tracking-[0.2em] text-sky-100">
              <Sparkles className="h-3.5 w-3.5" />
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
                <Link to="/uauth/apps/new">{t("dashboard.createApplication")}</Link>
              </Button>
              <Button
                asChild
                size="lg"
                variant="outline"
                className="border-white/20 bg-white/5 text-white hover:bg-white/10 hover:text-white"
              >
                <Link to="/uauth/apps">
                  {t("dashboard.openApplications")}
                  <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
            {personalCards.map((card) => (
              <div key={card.title} className="rounded-2xl border border-white/10 bg-white/8 p-4 backdrop-blur">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm text-slate-200">{card.title}</p>
                    <p className="text-3xl font-semibold">{card.value}</p>
                  </div>
                  <card.icon className="h-5 w-5 text-sky-200" />
                </div>
              </div>
            ))}
            <div className="rounded-2xl border border-dashed border-white/20 bg-slate-950/20 p-4 text-sm text-slate-200 sm:col-span-2 lg:col-span-1">
              <p className="font-medium text-white">{t("dashboard.setupProgressTitle")}</p>
              <p className="mt-1 text-slate-300">
                {t("dashboard.setupProgressDescription", {
                  completed: completedSetupCount,
                  total: setupSteps.length,
                })}
              </p>
            </div>
          </div>
        </div>
      </section>

      <Card className="rounded-3xl">
        <CardHeader>
          <CardTitle>{t("dashboard.quickAccessTitle")}</CardTitle>
          <CardDescription>{t("dashboard.quickAccessDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
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

      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <Card className="rounded-3xl">
          <CardHeader>
            <CardTitle>{t("dashboard.setupTitle")}</CardTitle>
            <CardDescription>{t("dashboard.setupDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {setupSteps.map((step) => (
              <div
                key={step.title}
                className="flex flex-col gap-4 rounded-2xl border bg-muted/20 p-4 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="flex items-start gap-3">
                  <div className="mt-0.5 rounded-full bg-muted p-1">
                    <CheckCircle2
                      className={step.complete ? "h-4 w-4 text-emerald-600" : "h-4 w-4 text-muted-foreground"}
                    />
                  </div>
                  <div className="space-y-1">
                    <p className="font-medium">{step.title}</p>
                    <p className="text-sm text-muted-foreground">{step.description}</p>
                  </div>
                </div>
                <Button asChild variant={step.complete ? "outline" : "default"} className="shrink-0">
                  <Link to={step.href}>{step.action}</Link>
                </Button>
              </div>
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

      <Card className="rounded-3xl">
        <CardHeader>
          <CardTitle>{t("dashboard.signInsTitle")}</CardTitle>
          <CardDescription>{t("dashboard.signInsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {userStats.app_stats.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("apps.name")}</TableHead>
                  <TableHead>{t("apps.clientId")}</TableHead>
                  <TableHead className="text-right">{t("dashboard.loginCount")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {userStats.app_stats.map((app) => (
                  <TableRow key={app.client_id}>
                    <TableCell className="font-medium">{app.app_name}</TableCell>
                    <TableCell className="font-mono text-xs">{app.client_id}</TableCell>
                    <TableCell className="text-right">{app.login_count}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <Empty className="border bg-muted/20 py-10">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <AppWindow className="h-5 w-5" />
                </EmptyMedia>
                <EmptyTitle>{t("dashboard.emptyTitle")}</EmptyTitle>
                <EmptyDescription>{t("dashboard.emptyDescription")}</EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button asChild>
                  <Link to="/uauth/apps/new">{t("dashboard.createFirstApplication")}</Link>
                </Button>
              </EmptyContent>
            </Empty>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
