import { useAuthStore } from '../stores/authStore'
import type { UserRole } from '../types'

// Checks if the current user has the required role.
export function canAccess(requiredRole: UserRole): boolean {
  const { user } = useAuthStore.getState()
  if (!user) return false
  return user.role === requiredRole
}
