import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Coins, CalendarCheck, Loader2, TrendingUp, ChevronLeft, ChevronRight, Mail } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { pointsApi, authApi } from "@/api/services"
import { toast } from "sonner"
import { getApiErrorKind, getErrorMessage, type ApiErrorKind } from "@/api/client"
import { useAuthStore } from "@/stores/auth"
import PageErrorState from "@/components/page-error-state"

const levelThresholds = [0, 0, 200, 800, 2400, 6000, 15000]

interface PointData {
  points: {
    paid_balance: number
    free_balance: number
    total_earned: number
    level: number
  }
  total_balance: number
}

interface Transaction {
  id: string
  kind: string
  amount: number
  balance: number
  reason: string
  created_at: string
}

interface CheckInStatus {
  checked_in_today: boolean
  streak_days: number
  monthly_records: Array<{ check_in_date: string; points_awarded: number }>
}

const WEEKDAYS = ["一", "二", "三", "四", "五", "六", "日"]

function CalendarGrid({ year, month, records, onPrev, onNext, disableNext }: {
  year: number
  month: number
  records: Array<{ check_in_date: string; points_awarded: number }>
  onPrev: () => void
  onNext: () => void
  disableNext: boolean
}) {
  const { t } = useTranslation()
  const today = new Date()
  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`

  // Build lookup: "YYYY-MM-DD" → points
  const checkedMap = new Map<string, number>()
  for (const r of records) {
    checkedMap.set(r.check_in_date, r.points_awarded)
  }

  // First day of month (0=Sun), days in month
  const firstDay = new Date(year, month - 1, 1).getDay()
  const daysInMonth = new Date(year, month, 0).getDate()
  // Convert Sunday=0 to Monday-based offset (Mon=0, Sun=6)
  const startOffset = firstDay === 0 ? 6 : firstDay - 1

  // Build 6×7 grid cells
  const cells: Array<{ day: number; dateStr: string } | null> = []
  for (let i = 0; i < startOffset; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${year}-${String(month).padStart(2, "0")}-${String(d).padStart(2, "0")}`
    cells.push({ day: d, dateStr })
  }
  while (cells.length % 7 !== 0) cells.push(null)

  return (
    <div>
      {/* Month navigation */}
      <div className="flex items-center justify-center gap-4 mb-3">
        <Button variant="ghost" size="icon" onClick={onPrev} className="h-8 w-8">
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <span className="text-sm font-medium min-w-[120px] text-center">
          {year}{t("common.year", "年")}{month}{t("common.month", "月")}
        </span>
        <Button variant="ghost" size="icon" onClick={onNext} disabled={disableNext} className="h-8 w-8">
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      {/* Weekday headers */}
      <div className="grid grid-cols-7 gap-1 mb-1">
        {WEEKDAYS.map((d) => (
          <div key={d} className="text-center text-xs text-muted-foreground font-medium py-1">{d}</div>
        ))}
      </div>

      {/* Day cells */}
      <div className="grid grid-cols-7 gap-1">
        {cells.map((cell, i) => {
          if (!cell) return <div key={`empty-${i}`} className="h-10" />
          const checked = checkedMap.get(cell.dateStr)
          const isToday = cell.dateStr === todayStr
          const isChecked = checked !== undefined

          return (
            <div
              key={cell.dateStr}
              title={isChecked ? t("points.pointsAwarded", { points: checked }) : undefined}
              className={[
                "h-10 rounded-md flex flex-col items-center justify-center text-xs transition-colors",
                isChecked
                  ? "bg-primary text-primary-foreground font-medium"
                  : isToday
                    ? "border-2 border-primary font-medium"
                    : "text-muted-foreground hover:bg-muted",
              ].join(" ")}
            >
              <span>{cell.day}</span>
              {isChecked && <span className="text-[10px] opacity-80">+{checked}</span>}
            </div>
          )
        })}
      </div>
    </div>
  )
}

export default function PointsPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const [loading, setLoading] = useState(true)
  const [errorKind, setErrorKind] = useState<ApiErrorKind | null>(null)
  const [emailVerifyDialogOpen, setEmailVerifyDialogOpen] = useState(false)
  const [resendingEmail, setResendingEmail] = useState(false)
  const [data, setData] = useState<PointData | null>(null)
  const [status, setStatus] = useState<CheckInStatus | null>(null)
  const [txns, setTxns] = useState<Transaction[]>([])
  const [txnTotal, setTxnTotal] = useState(0)
  const [checkingIn, setCheckingIn] = useState(false)
  const [page, setPage] = useState(1)

  const now = new Date()
  const [calYear, setCalYear] = useState(now.getFullYear())
  const [calMonth, setCalMonth] = useState(now.getMonth() + 1)

  const fetchData = useCallback(async () => {
    setLoading(true)
    setErrorKind(null)
    try {
      const [pRes, sRes, tRes] = await Promise.all([
        pointsApi.getMyPoints(),
        pointsApi.getCheckInStatus(calYear, calMonth),
        pointsApi.getTransactions(page),
      ])
      setData(pRes.data)
      setStatus(sRes.data)
      setTxns(tRes.data.transactions ?? [])
      setTxnTotal(tRes.data.total ?? 0)
    } catch (error) {
      setErrorKind(getApiErrorKind(error))
    } finally {
      setLoading(false)
    }
  }, [calYear, calMonth, page])

  useEffect(() => { fetchData() }, [fetchData])

  const handleCheckIn = async () => {
    setCheckingIn(true)
    try {
      await pointsApi.checkIn()
      await fetchData()
      toast.success(t("points.checkInSuccess"))
    } catch (error) {
      const errorMsg = getErrorMessage(error, "")
      if (errorMsg === "email_not_verified") {
        setEmailVerifyDialogOpen(true)
      } else {
        toast.error(getErrorMessage(error, t("common.error")))
      }
    } finally {
      setCheckingIn(false)
    }
  }

  const handleResendVerification = async () => {
    setResendingEmail(true)
    try {
      await authApi.resendVerification()
      toast.success(t("profile.resendSuccess"))
      setEmailVerifyDialogOpen(false)
    } catch (error) {
      toast.error(getErrorMessage(error, t("common.error")))
    } finally {
      setResendingEmail(false)
    }
  }

  if (loading) {
    return <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
  }

  if (errorKind) {
    return <PageErrorState kind={errorKind} onRetry={() => void fetchData()} />
  }

  const pts = data?.points
  const level = pts?.level ?? 1
  const nextThreshold = level < 6 ? levelThresholds[level + 1] : null
  const earned = pts?.total_earned ?? 0

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">{t("points.title")}</h1>

      {/* Balance cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">{t("points.totalBalance")}</CardTitle>
            <Coins className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{data?.total_balance ?? 0}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">{t("points.freeBalance")}</CardTitle>
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{pts?.free_balance ?? 0}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">{t("points.paidBalance")}</CardTitle>
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{pts?.paid_balance ?? 0}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">{t("points.level")}</CardTitle>
            <TrendingUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">Lv.{level}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {nextThreshold ? t("points.levelProgress", { needed: nextThreshold - earned }) : t("points.maxLevel")}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Level progress bar */}
      {nextThreshold && (
        <div className="w-full bg-secondary rounded-full h-2">
          <div
            className="bg-primary h-2 rounded-full transition-all"
            style={{ width: `${Math.min(100, (earned / nextThreshold) * 100)}%` }}
          />
        </div>
      )}

      {/* Check-in Calendar */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <CalendarCheck className="h-5 w-5" />
            {t("points.calendar")}
          </CardTitle>
          <Button onClick={handleCheckIn} disabled={checkingIn || status?.checked_in_today}>
            {checkingIn ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
            {status?.checked_in_today ? t("points.alreadyCheckedIn") : t("points.checkIn")}
          </Button>
        </CardHeader>
        <CardContent>
          <CalendarGrid
            year={calYear}
            month={calMonth}
            records={status?.monthly_records ?? []}
            onPrev={() => {
              if (calMonth === 1) { setCalYear(calYear - 1); setCalMonth(12) }
              else setCalMonth(calMonth - 1)
            }}
            onNext={() => {
              const isCurrentMonth = calYear === now.getFullYear() && calMonth === now.getMonth() + 1
              if (isCurrentMonth) return
              if (calMonth === 12) { setCalYear(calYear + 1); setCalMonth(1) }
              else setCalMonth(calMonth + 1)
            }}
            disableNext={calYear === now.getFullYear() && calMonth === now.getMonth() + 1}
          />
          <p className="text-sm text-muted-foreground mt-4">
            {t("points.streakDays", { days: status?.streak_days ?? 0 })}
          </p>
        </CardContent>
      </Card>

      {/* Transactions */}
      <Card>
        <CardHeader>
          <CardTitle>{t("points.transactions")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("points.kind")}</TableHead>
                <TableHead>{t("points.amount")}</TableHead>
                <TableHead>{t("points.balance")}</TableHead>
                <TableHead>{t("points.reason")}</TableHead>
                <TableHead>{t("points.time")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {txns.map((tx) => (
                <TableRow key={tx.id}>
                  <TableCell>{t(`points.${tx.kind}`, tx.kind)}</TableCell>
                  <TableCell className={tx.amount > 0 ? "text-green-600" : "text-red-600"}>
                    {tx.amount > 0 ? `+${tx.amount}` : tx.amount}
                  </TableCell>
                  <TableCell>{tx.balance}</TableCell>
                  <TableCell>{tx.reason || "-"}</TableCell>
                  <TableCell>{new Date(tx.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
              {txns.length === 0 && (
                <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">{t("common.noData")}</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
          {txnTotal > 20 && (
            <div className="flex justify-center gap-2 mt-4">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>Prev</Button>
              <span className="text-sm py-1">{page} / {Math.ceil(txnTotal / 20)}</span>
              <Button variant="outline" size="sm" disabled={page >= Math.ceil(txnTotal / 20)} onClick={() => setPage(page + 1)}>Next</Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Email Verification Dialog */}
      <Dialog open={emailVerifyDialogOpen} onOpenChange={setEmailVerifyDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Mail className="h-6 w-6" />
            </div>
            <DialogTitle className="text-center">{t("points.emailVerificationRequired")}</DialogTitle>
            <DialogDescription className="text-center">
              {t("points.emailVerificationDesc", { email: user?.email })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex-col sm:flex-row gap-2">
            <Button variant="outline" onClick={() => setEmailVerifyDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleResendVerification} disabled={resendingEmail}>
              {resendingEmail && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t("profile.resendVerification")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
