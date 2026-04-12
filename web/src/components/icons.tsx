import { IconBrandGithubFilled, IconBrandGoogleFilled } from "@tabler/icons-react"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

type BrandIconProps = ComponentProps<typeof IconBrandGithubFilled>

export function GitHubIcon({ className, ...props }: BrandIconProps) {
  return (
    <IconBrandGithubFilled
      aria-hidden="true"
      className={cn("shrink-0", className)}
      {...props}
    />
  )
}

export function GoogleIcon({ className, ...props }: BrandIconProps) {
  return (
    <IconBrandGoogleFilled
      aria-hidden="true"
      className={cn("shrink-0", className)}
      {...props}
    />
  )
}
