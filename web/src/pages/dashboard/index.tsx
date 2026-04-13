import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import dayjs from "dayjs"
import {
  ArrowRight,
  Coins,
  KeyRound,
  Loader2,
  ShieldCheck,
  UserCircle,
  Users,
} from "lucide-react"
import { statsApi, userApi } from "@/api/services"
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

interface AuthorizedApp {
  id: string
  client_id: string
  client_name: string
  scopes: string
  granted_at: string
}

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
    total_clients: 0,
    total_logins: 0,
    today_logins: 0,
  })
  const [authorizedApps, setAuthorizedApps] = useState<{ apps: AuthorizedApp[]; total: number }>({
    apps: [],
    total: 0,
  })

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      setErrorKind(null)

      try {
        const [authorizedRes, adminRes] = await Promise.all([
          userApi.listAuthorizedApps(1, 5),
          canReadAdminStats ? statsApi.admin() : Promise.resolve(null),
        ])

        setAuthorizedApps({
          apps: authorizedRes.data.apps ?? [],
          total: authorizedRes.data.total ?? 0,
        })

        if (adminRes) {
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
    { title: t("dashboard.platformApplications"), value: adminStats.total_clients, icon: KeyRound },
    { title: t("dashboard.platformLogins"), value: adminStats.total_logins, icon: ShieldCheck },
    { title: t("dashboard.platformToday"), value: adminStats.today_logins, icon: ArrowRight },
  ]

  const quickActions: QuickAction[] = [
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

  const secondaryAction = canAccessAdmin && adminEntry
    ? { href: adminEntry, label: t("app.admin") }
    : { href: "/account/points", label: t("points.title") }

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
                <Link to="/account/profile">{t("app.profile")}</Link>
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
            <div className="rounded-2xl border border-white/10 bg-white/8 p-4 backdrop-blur">
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <p className="text-sm text-slate-200">{t("profile.authorizedApps")}</p>
                  <p className="text-3xl font-semibold">{authorizedApps.total}</p>
                </div>
                <KeyRound className="h-5 w-5 text-sky-200" />
              </div>
            </div>
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

      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <Card className="rounded-3xl">
          <CardHeader>
            <CardTitle>{t("profile.authorizedApps")}</CardTitle>
            <CardDescription>{t("profile.authorizedAppsDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            {authorizedApps.apps.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("profile.authorizedAppsApp")}</TableHead>
                    <TableHead>{t("profile.authorizedAppsClientId")}</TableHead>
                    <TableHead>{t("profile.authorizedAppsScopes")}</TableHead>
                    <TableHead className="text-right">{t("profile.authorizedAppsGrantedAt")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {authorizedApps.apps.map((app) => (
                    <TableRow key={app.id}>
                      <TableCell className="font-medium">{app.client_name || app.client_id}</TableCell>
                      <TableCell className="font-mono text-xs">{app.client_id}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{app.scopes || "-"}</TableCell>
                      <TableCell className="text-right text-sm text-muted-foreground">
                        {app.granted_at ? dayjs(app.granted_at).format("YYYY-MM-DD HH:mm") : "-"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <Empty className="border bg-muted/20 py-10">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <KeyRound className="h-5 w-5" />
                  </EmptyMedia>
                  <EmptyTitle>{t("profile.authorizedAppsEmpty")}</EmptyTitle>
                  <EmptyDescription>{t("profile.authorizedAppsDesc")}</EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <Button asChild>
                    <Link to="/account/profile">{t("app.profile")}</Link>
                  </Button>
                </EmptyContent>
              </Empty>
            )}
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
    </div>
  )
}
