import { useEffect } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { Loader2 } from "lucide-react"
import { authApi } from "@/api/services"
import { useAuthStore } from "@/stores/auth"

export default function LoginCallbackPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { setUser } = useAuthStore()

  useEffect(() => {
    const code = searchParams.get("code")
    if (!code) {
      navigate("/login", { replace: true })
      return
    }
    authApi.socialExchange(code).then((res) => {
      // Tokens are set in HttpOnly cookies
      setUser(res.data.user)
      navigate("/", { replace: true })
    }).catch(() => {
      navigate("/login", { replace: true })
    })
  }, [searchParams, navigate, setUser])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
    </div>
  )
}
