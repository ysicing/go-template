import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { PropsWithChildren } from "react"
import { useMemo } from "react"

import "@/locales"

export function AppProviders({ children }: PropsWithChildren) {
  const queryClient = useMemo(() => new QueryClient(), [])
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
