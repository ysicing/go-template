import { create } from "zustand"

export interface User {
  id: number
  username: string
  email: string
  role: "user" | "admin"
  is_admin: boolean
}

export type AuthInitStatus =
  | "pending"
  | "ready"
  | "unauthenticated"
  | "service_unavailable"
  | "setup_required"

interface AuthState {
  user: User | null
  initStatus: AuthInitStatus
  setUser: (user: Omit<User, "is_admin"> | User) => void
  setInitStatus: (status: AuthInitStatus) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()((set) => ({
  user: null,
  initStatus: "pending",
  setUser: (user) =>
    set({
      user: {
        ...user,
        is_admin: "is_admin" in user ? user.is_admin : user.role === "admin",
      },
      initStatus: "ready",
    }),
  setInitStatus: (status) => set({ initStatus: status }),
  logout: () => set({ user: null, initStatus: "unauthenticated" }),
}))
