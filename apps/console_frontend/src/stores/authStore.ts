import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AuthUser, UserRole } from '../types'
import { parseJwt } from '../lib/jwt'

interface AuthState {
  user: AuthUser | null
  login: (token: string, role?: UserRole, email?: string) => void
  logout: () => void
  isLoggedIn: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,

      login: (token: string, role?: UserRole, email?: string) => {
        const payload = parseJwt(token)
        const payloadRole = String(payload?.role ?? '').toLowerCase()
        const tokenRole =
          role || (payloadRole === 'admin' || payloadRole === 'guest' ? (payloadRole as UserRole) : 'guest')
        const normalizedEmail = (email || String(payload?.email ?? '')).trim().toLowerCase() || undefined
        
        const tokenPreview =
          token.length > 10
            ? `${token.slice(0, 6)}...${token.slice(-4)}`
            : `${token.slice(0, 6)}...`
        set({
          user: {
            token,
            role: tokenRole,
            tokenPreview,
            expiresAt: payload?.exp ? payload.exp * 1000 : null,
            email: normalizedEmail,
          },
        })
      },

      logout: () => {
        set({ user: null })
      },

      isLoggedIn: () => {
        const user = get().user
        if (!user) return false
        if (user.expiresAt && user.expiresAt <= Date.now()) {
          set({ user: null })
          return false
        }
        return true
      },
    }),
    {
      name: 'vbs-auth',
      partialize: (s) => ({ user: s.user }),
    }
  )
)
