import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Loader2 } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { adminPointsApi } from "@/api/services"
import {
  Tabs, TabsContent, TabsList, TabsTrigger,
} from "@/components/ui/tabs"
import { useHasPermission, adminPermissions } from "@/lib/permissions"

interface Transaction {
  id: string
  user_id: string
  kind: string
  amount: number
  balance: number
  reason: string
  created_at: string
}

interface LeaderboardEntry {
  user_id: string
  total_earned: number
  level: number
}

export default function AdminPointsPage() {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.pointsRead)
  const canWrite = useHasPermission(adminPermissions.pointsWrite)
  const [loading, setLoading] = useState(false)
  const [adjustForm, setAdjustForm] = useState({
    user_id: "", point_type: "free", amount: 0, reason: "",
  })
  const [txns, setTxns] = useState<Transaction[]>([])
  const [txnTotal, setTxnTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [searchId, setSearchId] = useState("")
  const [searchResult, setSearchResult] = useState<{ points: { paid_balance: number; free_balance: number; total_earned: number; level: number }; total_balance: number } | null>(null)

  const fetchTxns = useCallback(async () => {
    if (!canRead) {
      setTxns([])
      setTxnTotal(0)
      return
    }
    const res = await adminPointsApi.getTransactions(page)
    setTxns(res.data.transactions ?? [])
    setTxnTotal(res.data.total ?? 0)
  }, [canRead, page])

  const fetchLeaderboard = useCallback(async () => {
    if (!canRead) {
      setLeaderboard([])
      return
    }
    const res = await adminPointsApi.getLeaderboard()
    setLeaderboard(res.data.leaderboard ?? [])
  }, [canRead])

  useEffect(() => {
    if (canRead) {
      fetchTxns()
    }
  }, [canRead, fetchTxns])

  useEffect(() => {
    if (canRead) {
      fetchLeaderboard()
    }
  }, [canRead, fetchLeaderboard])

  const handleAdjust = async () => {
    if (!adjustForm.user_id || adjustForm.amount === 0 || !canWrite) return
    setLoading(true)
    try {
      await adminPointsApi.adjust(adjustForm)
      setAdjustForm({ user_id: "", point_type: "free", amount: 0, reason: "" })
      await fetchTxns()
      await fetchLeaderboard()
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = async () => {
    if (!searchId || !canRead) return
    try {
      const res = await adminPointsApi.getUserPoints(searchId)
      setSearchResult(res.data)
    } catch {
      setSearchResult(null)
    }
  }

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">{t("adminPoints.title")}</h1>

      <Tabs defaultValue="adjust">
        <TabsList>
          {canWrite && <TabsTrigger value="adjust">{t("adminPoints.adjust")}</TabsTrigger>}
          <TabsTrigger value="search">{t("adminPoints.search")}</TabsTrigger>
          <TabsTrigger value="transactions">{t("adminPoints.allTransactions")}</TabsTrigger>
          <TabsTrigger value="leaderboard">{t("adminPoints.leaderboard")}</TabsTrigger>
        </TabsList>

        {canWrite && (
          <TabsContent value="adjust">
            <Card>
              <CardHeader><CardTitle>{t("adminPoints.adjust")}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t("adminPoints.userId")}</Label>
                    <Input value={adjustForm.user_id} onChange={(e) => setAdjustForm({ ...adjustForm, user_id: e.target.value })} />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("adminPoints.pointType")}</Label>
                    <select
                      className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                      value={adjustForm.point_type}
                      onChange={(e) => setAdjustForm({ ...adjustForm, point_type: e.target.value })}
                    >
                      <option value="free">{t("adminPoints.free")}</option>
                      <option value="paid">{t("adminPoints.paid")}</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label>{t("adminPoints.amount")}</Label>
                    <Input type="number" value={adjustForm.amount} onChange={(e) => setAdjustForm({ ...adjustForm, amount: parseInt(e.target.value) || 0 })} />
                    <p className="text-xs text-muted-foreground">{t("adminPoints.amountHint")}</p>
                  </div>
                  <div className="space-y-2">
                    <Label>{t("adminPoints.reason")}</Label>
                    <Input value={adjustForm.reason} onChange={(e) => setAdjustForm({ ...adjustForm, reason: e.target.value })} />
                  </div>
                </div>
                <Button onClick={handleAdjust} disabled={loading || !canWrite}>
                  {loading && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
                  {t("adminPoints.adjust")}
                </Button>
              </CardContent>
            </Card>
          </TabsContent>
        )}

        <TabsContent value="search">
          <Card>
            <CardHeader><CardTitle>{t("adminPoints.search")}</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="flex gap-2">
                <Input placeholder={t("adminPoints.searchPlaceholder")} value={searchId} onChange={(e) => setSearchId(e.target.value)} />
                <Button onClick={handleSearch}>{t("adminPoints.search")}</Button>
              </div>
              {searchResult && (
                <div className="grid gap-2 md:grid-cols-4 mt-4">
                  <div className="text-sm"><span className="text-muted-foreground">{t("points.freeBalance")}:</span> {searchResult.points.free_balance}</div>
                  <div className="text-sm"><span className="text-muted-foreground">{t("points.paidBalance")}:</span> {searchResult.points.paid_balance}</div>
                  <div className="text-sm"><span className="text-muted-foreground">{t("points.totalEarned")}:</span> {searchResult.points.total_earned}</div>
                  <div className="text-sm"><span className="text-muted-foreground">{t("points.level")}:</span> Lv.{searchResult.points.level}</div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="transactions">
          <Card>
            <CardHeader><CardTitle>{t("adminPoints.allTransactions")}</CardTitle></CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("adminPoints.userId")}</TableHead>
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
                      <TableCell className="font-mono text-xs">{tx.user_id.slice(0, 8)}...</TableCell>
                      <TableCell>{tx.kind}</TableCell>
                      <TableCell className={tx.amount > 0 ? "text-green-600" : "text-red-600"}>
                        {tx.amount > 0 ? `+${tx.amount}` : tx.amount}
                      </TableCell>
                      <TableCell>{tx.balance}</TableCell>
                      <TableCell>{tx.reason || "-"}</TableCell>
                      <TableCell>{new Date(tx.created_at).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                  {txns.length === 0 && (
                    <TableRow><TableCell colSpan={6} className="text-center text-muted-foreground">{t("common.noData")}</TableCell></TableRow>
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
        </TabsContent>

        <TabsContent value="leaderboard">
          <Card>
            <CardHeader><CardTitle>{t("adminPoints.leaderboard")}</CardTitle></CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("adminPoints.rank")}</TableHead>
                    <TableHead>{t("adminPoints.userId")}</TableHead>
                    <TableHead>{t("adminPoints.totalEarned")}</TableHead>
                    <TableHead>{t("adminPoints.level")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {leaderboard.map((entry, i) => (
                    <TableRow key={entry.user_id}>
                      <TableCell>{i + 1}</TableCell>
                      <TableCell className="font-mono text-xs">{entry.user_id.slice(0, 8)}...</TableCell>
                      <TableCell>{entry.total_earned}</TableCell>
                      <TableCell>Lv.{entry.level}</TableCell>
                    </TableRow>
                  ))}
                  {leaderboard.length === 0 && (
                    <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">{t("common.noData")}</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
