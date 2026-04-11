import axios from "axios"
import { useAuthStore } from "@/stores/auth"

const api = axios.create({
  baseURL: "/api",
  timeout: 10000,
  withCredentials: true  // Enable sending cookies with requests (required for HttpOnly cookies)
})

// Note: We no longer set Authorization header manually
// Tokens are sent automatically via HttpOnly cookies

let refreshInFlight: Promise<boolean> | null = null
let isRefreshing = false

async function refreshAccessToken(): Promise<boolean> {
  // If a refresh is already in progress, wait for it
  if (refreshInFlight) {
    return refreshInFlight
  }

  // Double-check with flag to prevent race condition
  if (isRefreshing) {
    // Wait a bit and retry
    await new Promise(resolve => setTimeout(resolve, 50))
    if (refreshInFlight) {
      return refreshInFlight
    }
  }

  isRefreshing = true
  refreshInFlight = axios.post("/api/auth/refresh", {}, { withCredentials: true })
    .then(() => {
      // Tokens are automatically updated in cookies by the server
      return true
    })
    .catch((err: unknown) => {
      if (axios.isAxiosError(err) && (err.response?.status === 401 || err.response?.status === 403)) {
        console.error('[Auth] Refresh token failed:', err.response?.status, err.response?.data)
        useAuthStore.getState().logout()
        if (window.location.pathname !== "/login") {
          window.location.href = "/login"
        }
      }
      return false
    })
    .finally(() => {
      // Add delay before clearing to ensure all waiting requests see the result
      setTimeout(() => {
        refreshInFlight = null
        isRefreshing = false
      }, 100)
    })

  return refreshInFlight
}

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config
    if (error.response?.status === 401 && original && !original._retry) {
      original._retry = true

      // CRITICAL: Wait for any in-flight refresh to complete first
      // This prevents multiple concurrent 401s from triggering parallel refresh requests
      // which would cause token replay detection and logout
      const success = await refreshAccessToken()
      if (success) {
        // Add a small delay to ensure cookies are updated by the browser
        // before retrying the original request
        await new Promise(resolve => setTimeout(resolve, 100))
        return api(original)
      }
    }
    return Promise.reject(error)
  },
)

export default api

export type ApiErrorKind =
  | "unauthorized"
  | "forbidden"
  | "not_found"
  | "server"
  | "network"
  | "timeout"
  | "unknown"

export function getApiErrorKind(err: unknown): ApiErrorKind {
  if (!axios.isAxiosError(err)) {
    return "unknown"
  }

  if (err.code === "ECONNABORTED") {
    return "timeout"
  }

  const status = err.response?.status
  if (status === 401) return "unauthorized"
  if (status === 403) return "forbidden"
  if (status === 404) return "not_found"
  if (status !== undefined && status >= 500) return "server"
  if (!status) return "network"

  return "unknown"
}

/** Extract error message from API response, fallback to default */
export function getErrorMessage(err: unknown, fallback: string): string {
  if (
    err &&
    typeof err === "object" &&
    "response" in err &&
    (err as { response?: { data?: { error?: string } } }).response?.data?.error
  ) {
    return (err as { response: { data: { error: string } } }).response.data.error
  }
  return fallback
}
