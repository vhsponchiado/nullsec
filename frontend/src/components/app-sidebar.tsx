import type { LucideIcon } from 'lucide-react'
import { History, ArrowLeftRight, Send, ShieldCheck, Bug, Syringe, Binary, Crosshair, Radar, Globe, Waypoints, Code2, Network, ScanEye } from 'lucide-react'
import { motion } from 'framer-motion'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export type NavKey =
  | 'history'
  | 'intercept'
  | 'repeater'
  | 'intruder'
  | 'decoder'
  | 'port-scanner'
  | 'subdomain-finder'
  | 'spider'
  | 'passive-scan'
  | 'certificate'
  | 'settings'
  | 'wordpress-check'
  | 'sqli-check'
  | 'xss-check'

interface NavItem {
  key: NavKey
  label: string
  icon: LucideIcon
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Proxy',
    items: [
      { key: 'history', label: 'Proxy History', icon: History },
      { key: 'intercept', label: 'Intercept', icon: ArrowLeftRight },
      { key: 'repeater', label: 'Repeater', icon: Send },
      { key: 'intruder', label: 'Intruder', icon: Crosshair },
    ],
  },
  {
    label: 'Recon',
    items: [
      { key: 'spider', label: 'Spider', icon: Network },
      { key: 'passive-scan', label: 'Passive Scan', icon: ScanEye },
      { key: 'port-scanner', label: 'Port Scanner', icon: Radar },
      { key: 'subdomain-finder', label: 'Subdomain Finder', icon: Globe },
      { key: 'wordpress-check', label: 'WordPress Check', icon: Bug },
      { key: 'sqli-check', label: 'SQL Injection', icon: Syringe },
      { key: 'xss-check', label: 'XSS', icon: Code2 },
    ],
  },
  {
    label: 'Tools',
    items: [{ key: 'decoder', label: 'Decoder', icon: Binary }],
  },
  {
    label: 'System',
    items: [
      { key: 'certificate', label: 'Certificate', icon: ShieldCheck },
      { key: 'settings', label: 'Settings', icon: Waypoints },
    ],
  },
]

interface AppSidebarProps {
  active: NavKey
  onSelect: (key: NavKey) => void
}

export function AppSidebar({ active, onSelect }: AppSidebarProps) {
  return (
    <nav className="w-16 shrink-0 border-r border-sidebar-border bg-sidebar flex flex-col items-center py-4 overflow-y-auto">
      {NAV_GROUPS.map((group, i) => (
        <div
          key={group.label}
          className={`flex flex-col items-center gap-1 w-full ${
            i > 0 ? 'mt-3 pt-3 border-t border-sidebar-border/60' : ''
          }`}
        >
          <span className="text-[9px] font-semibold tracking-widest text-sidebar-foreground/35 select-none mb-0.5">
            {group.label.toUpperCase()}
          </span>
          {group.items.map((item) => {
            const Icon = item.icon
            const isActive = active === item.key
            return (
              <Tooltip key={item.key}>
                <TooltipTrigger asChild>
                  <button
                    onClick={() => onSelect(item.key)}
                    aria-label={item.label}
                    className={`relative flex items-center justify-center w-11 h-11 rounded-lg cursor-pointer transition-colors ${
                      isActive
                        ? 'text-sidebar-primary-foreground'
                        : 'text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'
                    }`}
                  >
                    {isActive && (
                      <motion.span
                        layoutId="sidebar-active-indicator"
                        className="absolute inset-0 rounded-lg bg-sidebar-primary"
                        transition={{ type: 'spring', stiffness: 500, damping: 34 }}
                      />
                    )}
                    <Icon size={20} strokeWidth={2} className="relative z-10" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">{item.label}</TooltipContent>
              </Tooltip>
            )
          })}
        </div>
      ))}
    </nav>
  )
}
