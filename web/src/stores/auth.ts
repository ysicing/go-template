import { create } from "zustand"

export interface User {
  id: string
  username: string
  email: string
  is_admin: boolean
  permissions?: string[]
  email_verified: boolean
  avatar_url?: string
  provider?: string
  created_at?: string
}

export type AuthInitStatus =
  | "pending"
  | "ready"
  | "unauthenticated"
  | "service_unavailable"
  | "not_found"

interface AuthState {
  user: User | null
  initStatus: AuthInitStatus
  setUser: (user: User) => void
  setInitStatus: (status: AuthInitStatus) => void
  updateUser: (partial: Partial<User>) => void
  logout: () => void
}

// Note: Tokens are stored in HttpOnly cookies for security (XSS protection)
// We no longer store tokens in localStorage
export const useAuthStore = create<AuthState>()((set) => ({
  user: null,
  initStatus: "pending",
  setUser: (user) => set({ user, initStatus: "ready" }),
  setInitStatus: (status) => set({ initStatus: status }),
  updateUser: (partial) => set((state) => ({
    user: state.user ? { ...state.user, ...partial } : null,
  })),
  logout: () => set({ user: null, initStatus: "unauthenticated" }),
}))
