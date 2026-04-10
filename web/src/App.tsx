import { Suspense, useEffect, useState } from "react"
import { BrowserRouter } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { AppProviders } from "@/app/providers"
import { fetchCurrentUser, fetchSetupStatus, hasAccessToken } from "@/lib/api"
import AppRouter from "@/router"
import { useAppStore } from "@/stores/app"
import { useAuthStore } from "@/stores/auth"

function AppInner() {
  const { themeMode, applyPrimaryColor } = useAppStore()
  const { setUser, setInitStatus } = useAuthStore()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", themeMode === "dark")
  }, [themeMode])

  useEffect(() => {
    applyPrimaryColor()
  }, [applyPrimaryColor])

  useEffect(() => {
    let cancelled = false

    const bootstrap = async () => {
      try {
        const setup = await fetchSetupStatus()
        if (cancelled) return
        if (setup.setup_required) {
          setInitStatus("setup_required")
          setReady(true)
          return
        }
        if (!hasAccessToken()) {
          setInitStatus("unauthenticated")
          setReady(true)
          return
        }
        const user = await fetchCurrentUser()
        if (cancelled) return
        setUser(user)
      } catch {
        if (cancelled) return
        setInitStatus(hasAccessToken() ? "service_unavailable" : "unauthenticated")
      } finally {
        if (!cancelled) {
          setReady(true)
        }
      }
    }

    void bootstrap()
    return () => {
      cancelled = true
    }
  }, [setInitStatus, setUser])

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return (
    <BrowserRouter>
      <Suspense
        fallback={
          <div className="flex min-h-screen items-center justify-center">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        }
      >
        <AppRouter />
      </Suspense>
      <Toaster richColors position="top-right" />
    </BrowserRouter>
  )
}

export default function App() {
  return (
    <AppProviders>
      <AppInner />
    </AppProviders>
  )
}
