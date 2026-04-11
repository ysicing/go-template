import { Suspense, useEffect } from "react"
import axios from "axios"
import { BrowserRouter } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { useAppStore } from "@/stores/app"
import { useAuthStore } from "@/stores/auth"
import { useTokenRefresh } from "@/hooks/useTokenRefresh"
import { SiteTitleController } from "@/components/SiteTitleController"
import { userApi } from "@/api/services"
import AppRouter from "./router"

export default function App() {
  const { themeMode, applyPrimaryColor } = useAppStore()
  const { setUser, setInitStatus } = useAuthStore()

  // Auto-refresh access token before expiry
  useTokenRefresh()

  // Initialize user state from cookies on app start
  useEffect(() => {
    let cancelled = false

    const initializeAuth = async () => {
      const maxAttempts = 3

      for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
        try {
          const res = await userApi.getMe()
          if (cancelled) {
            return
          }
          if (res.data.user) {
            setUser(res.data.user)
          } else {
            setInitStatus("unauthenticated")
          }
          return
        } catch (err) {
          if (cancelled) {
            return
          }

          if (axios.isAxiosError(err)) {
            const status = err.response?.status
            if (status === 401 || status === 403) {
              setInitStatus("unauthenticated")
              return
            }
            if (status === 404) {
              setInitStatus("not_found")
              return
            }
            if (status !== undefined && status < 500) {
              setInitStatus("service_unavailable")
              return
            }
          }

          if (attempt < maxAttempts) {
            await new Promise((resolve) => setTimeout(resolve, attempt * 800))
            continue
          }

          setInitStatus("service_unavailable")
          return
        }
      }
    }

    void initializeAuth()

    return () => {
      cancelled = true
    }
  }, [setUser, setInitStatus])

  useEffect(() => {
    document.documentElement.classList.toggle("dark", themeMode === "dark")
  }, [themeMode])

  useEffect(() => {
    applyPrimaryColor()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <BrowserRouter>
      <TooltipProvider>
        <SiteTitleController />
        <Suspense fallback={
          <div className="flex min-h-screen items-center justify-center">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          </div>
        }>
          <AppRouter />
        </Suspense>
        <Toaster richColors position="top-right" />
      </TooltipProvider>
    </BrowserRouter>
  )
}
