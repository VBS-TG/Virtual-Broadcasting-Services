import { useAuthStore } from '../stores/authStore'
import type { UserRole } from '../types'

// Checks if the current user has the required role (admin > operator)
export function canAccess(requiredRole: UserRole): boolean {
  const { user } = useAuthStore.getState();
  if (!user) return false;
  
  if (requiredRole === 'operator') {
    return user.role === 'admin' || user.role === 'operator';
  }
  
  return user.role === requiredRole;
}
