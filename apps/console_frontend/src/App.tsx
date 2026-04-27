import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import Sidebar from './components/Sidebar'
import BottomNav from './components/BottomNav'
import LoginPage from './pages/LoginPage'
import Dashboard from './pages/Dashboard'
import RuntimeConfig from './pages/RuntimeConfig'
import SwitcherPage from './pages/SwitcherPage'
import PopoutSwitcher from './pages/PopoutSwitcher'
import PopoutMultiview from './pages/PopoutMultiview'
import Telemetry from './pages/Telemetry'
import SystemHealth from './pages/SystemHealth'
import OperationLog from './pages/OperationLog'
import Settings from './pages/Settings'
import RentalSessions from './pages/RentalSessions'
import PipelinePage from './pages/PipelinePage'
import ShowControlPage from './pages/ShowControlPage'
import ToastContainer from './components/ToastContainer'
import { ShieldAlert } from 'lucide-react'
import './index.css'
import './App.css'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn())
  if (!isLoggedIn) return <Navigate to="/login" replace />
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user)
  if (!user) return <Navigate to="/login" replace />
  if (user.role !== 'admin') {
    return (
      <div className="flex flex-col items-center justify-center h-full w-full">
        <ShieldAlert className="w-24 h-24 text-vbs-pgm opacity-80 mb-6" />
        <h1 className="text-6xl font-black text-white tracking-tighter drop-shadow-md mb-2">403</h1>
        <h2 className="text-2xl font-black text-vbs-pgm mb-2 tracking-widest">ACCESS DENIED</h2>
        <p className="text-vbs-muted font-bold uppercase tracking-widest text-[12px] bg-white/5 px-4 py-2 rounded-lg">Admin Privileges Required</p>
      </div>
    )
  }
  return <>{children}</>
}

function AppLayout() {
  return (
    <div className="flex flex-col h-full w-full overflow-hidden bg-gradient-to-b from-[#070B1E] to-[#003D3A]">
      
      <div className="flex flex-1 overflow-hidden min-h-0">
        {/* Sidebar：lg 以上顯示 */}
        <div className="hidden lg:flex">
          <Sidebar />
        </div>
        <main className="flex-1 overflow-hidden min-h-0">
          <Routes>
            <Route path="/dashboard"   element={<Dashboard />} />
            <Route path="/runtime"     element={<RequireAdmin><RuntimeConfig /></RequireAdmin>} />
            <Route path="/pipeline"    element={<PipelinePage />} />
            <Route path="/show-config" element={<ShowControlPage />} />
            <Route path="/rental-sessions" element={<RequireAdmin><RentalSessions /></RequireAdmin>} />
            <Route path="/switcher"    element={<SwitcherPage />} />
            <Route path="/telemetry"   element={<Telemetry />} />
            <Route path="/system"      element={<SystemHealth />} />
            <Route path="/logs"        element={<OperationLog />} />
            <Route path="/settings"    element={<RequireAdmin><Settings /></RequireAdmin>} />
            <Route path="*"            element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </main>
      </div>
      {/* BottomNav：lg 以下顯示 */}
      <div className="flex lg:hidden">
        <BottomNav />
      </div>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <ToastContainer />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/popout/switcher" element={<RequireAuth><PopoutSwitcher /></RequireAuth>} />
        <Route path="/popout/multiviewer" element={<RequireAuth><PopoutMultiview /></RequireAuth>} />
        <Route
          path="/*"
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        />
      </Routes>
    </BrowserRouter>
  )
}
