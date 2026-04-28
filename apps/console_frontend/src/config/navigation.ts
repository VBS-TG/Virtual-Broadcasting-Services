import { LayoutDashboard, MonitorPlay, Activity, FileText, Network, KeyRound } from 'lucide-react'

export const NAV_ITEMS = [
  { path: '/dashboard',       label: '總覽', labelEn: 'Dashboard', icon: LayoutDashboard, desc: '系統總覽' },
  { path: '/switcher',        label: '導播', labelEn: 'Switcher',  icon: MonitorPlay,     desc: '切換控制' },
  { path: '/pipeline',        label: '鏈路', labelEn: 'Pipeline',  icon: Network,         desc: '鏈路監控' },
  { path: '/rental-sessions', label: '租賃', labelEn: 'Rentals',   icon: KeyRound,        desc: '租約管理' },
  { path: '/telemetry',       label: '遙測', labelEn: 'Telemetry', icon: Activity,        desc: '數據分析' },
  { path: '/logs',            label: '日誌', labelEn: 'Logs',      icon: FileText,        desc: '操作日誌' },
]
