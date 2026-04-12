import { getBuildVersionDetails, getBuildVersionLabel, type BuildVersionInfo } from "@/lib/build-version"
import { cn } from "@/lib/utils"

type BuildVersionProps = {
  info: BuildVersionInfo
  className?: string
}

export function BuildVersion({ info, className }: BuildVersionProps) {
  const label = getBuildVersionLabel(info)
  const details = getBuildVersionDetails(info)

  if (!label) {
    return null
  }

  return (
    <span className={cn(className)} title={details.join("\n") || undefined}>
      {label}
    </span>
  )
}
