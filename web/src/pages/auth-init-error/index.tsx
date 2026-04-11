import { useTranslation } from "react-i18next"
import { AlertTriangle, ServerCrash } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import type { AuthInitStatus } from "@/stores/auth"

export default function AuthInitErrorPage({ status }: { status: AuthInitStatus }) {
  const { t } = useTranslation()
  const isNotFound = status === "not_found"
  const Icon = isNotFound ? AlertTriangle : ServerCrash

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-6">
      <Empty className="max-w-2xl border bg-card shadow-sm">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Icon className="h-5 w-5" />
          </EmptyMedia>
          <EmptyTitle>
            {isNotFound ? t("authInit.notFoundTitle") : t("authInit.serviceUnavailableTitle")}
          </EmptyTitle>
          <EmptyDescription>
            {isNotFound ? t("authInit.notFoundDescription") : t("authInit.serviceUnavailableDescription")}
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent className="max-w-md">
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Button onClick={() => window.location.reload()}>
              {t("authInit.retry")}
            </Button>
            <Button variant="outline" onClick={() => { window.location.href = "/login" }}>
              {t("authInit.goLogin")}
            </Button>
          </div>
        </EmptyContent>
      </Empty>
    </div>
  )
}
