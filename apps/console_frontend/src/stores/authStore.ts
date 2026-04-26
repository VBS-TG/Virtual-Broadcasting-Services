import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AuthUser, UserRole } from '../types'

interface AuthState {
  user: AuthUser | null
  login: (token: string, role?: UserRole) => void
  logout: () => void
  isLoggedIn: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,

      login: (token: string, role: UserRole = 'admin') => {
        // [MOCK] 角色固定給 admin
        // TODO: 後端就緒後解析 JWT payload 取得 role & exp
        const tokenPreview =
          token.length > 10
            ? `${token.slice(0, 6)}...${token.slice(-4)}`
            : `${token.slice(0, 6)}...`
        set({ user: { token, role, tokenPreview, expiresAt: null } })
      },

      logout: () => {
        set({ user: null })
      },

      isLoggedIn: () => get().user !== null,
    }),
    {
      name: 'vbs-auth',
      partialize: (s) => ({ user: s.user }),
    }
  )
)
