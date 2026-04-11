import { useTranslation } from "react-i18next"
import { AlertTriangle, Ban, RefreshCw, SearchX, ServerCrash, ShieldAlert, WifiOff } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import type { ApiErrorKind } from "@/api/client"

const iconByKind = {
  unauthorized: ShieldAlert,
  forbidden: Ban,
  not_found: SearchX,
  server: ServerCrash,
  network: WifiOff,
  timeout: RefreshCw,
  unknown: AlertTriangle,
} satisfies Record<ApiErrorKind, React.ComponentType<{ className?: string }>>

export default function PageErrorState({
  kind,
  onRetry,
  compact = false,
}: {
  kind: ApiErrorKind
  onRetry?: () => void
  compact?: boolean
}) {
  const { t } = useTranslation()
  const Icon = iconByKind[kind]

  return (
    <Empty className={compact ? "border bg-card/50 py-10" : "border bg-card shadow-sm"}>
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Icon className="h-5 w-5" />
        </EmptyMedia>
        <EmptyTitle>{t(`apiErrors.${kind}.title`)}</EmptyTitle>
        <EmptyDescription>{t(`apiErrors.${kind}.description`)}</EmptyDescription>
      </EmptyHeader>
      {onRetry && (
        <EmptyContent className="max-w-sm">
          <Button onClick={onRetry}>{t("apiErrors.retry")}</Button>
        </EmptyContent>
      )}
    </Empty>
  )
}
