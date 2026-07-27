import type { LucideIcon } from 'lucide-react'
import { History, ArrowLeftRight, Send, ShieldCheck, Bug, Syringe, Binary, Crosshair, Radar, Globe, Waypoints, Code2, Network, ScanEye, FolderSearch, Info } from 'lucide-react'
import { motion } from 'framer-motion'
import { useTranslation } from 'react-i18next'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export type NavKey =
  | 'history'
  | 'intercept'
  | 'repeater'
  | 'intruder'
  | 'decoder'
  | 'port-scanner'
  | 'subdomain-finder'
  | 'dir-finder'
  | 'spider'
  | 'passive-scan'
  | 'certificate'
  | 'settings'
  | 'about'
  | 'wordpress-check'
  | 'sqli-check'
  | 'xss-check'

interface NavItem {
  key: NavKey
  icon: LucideIcon
}

type NavGroupKey = 'proxy' | 'recon' | 'tools' | 'system'

interface NavGroup {
  groupKey: NavGroupKey
  items: NavItem[]
}

const NAV_GROUPS: NavGroup[] = [
  {
    groupKey: 'proxy',
    items: [
      { key: 'history', icon: History },
      { key: 'intercept', icon: ArrowLeftRight },
      { key: 'repeater', icon: Send },
      { key: 'intruder', icon: Crosshair },
    ],
  },
  {
    groupKey: 'recon',
    items: [
      { key: 'spider', icon: Network },
      { key: 'passive-scan', icon: ScanEye },
      { key: 'port-scanner', icon: Radar },
      { key: 'subdomain-finder', icon: Globe },
      { key: 'dir-finder', icon: FolderSearch },
      { key: 'wordpress-check', icon: Bug },
      { key: 'sqli-check', icon: Syringe },
      { key: 'xss-check', icon: Code2 },
    ],
  },
  {
    groupKey: 'tools',
    items: [{ key: 'decoder', icon: Binary }],
  },
  {
    groupKey: 'system',
    items: [
      { key: 'certificate', icon: ShieldCheck },
      { key: 'settings', icon: Waypoints },
      { key: 'about', icon: Info },
    ],
  },
]

interface AppSidebarProps {
  active: NavKey
  onSelect: (key: NavKey) => void
}

export function AppSidebar({ active, onSelect }: AppSidebarProps) {
  const { t } = useTranslation('common')

  return (
    <nav className="w-16 shrink-0 border-r border-sidebar-border bg-sidebar flex flex-col items-center py-4 overflow-y-auto">
      {NAV_GROUPS.map((group, i) => (
        <div
          key={group.groupKey}
          className={`flex flex-col items-center gap-1 w-full ${
            i > 0 ? 'mt-3 pt-3 border-t border-sidebar-border/60' : ''
          }`}
        >
          <span className="text-[9px] font-semibold tracking-widest text-sidebar-foreground/35 select-none mb-0.5">
            {t(`navGroups.${group.groupKey}`).toUpperCase()}
          </span>
          {group.items.map((item) => {
            const Icon = item.icon
            const isActive = active === item.key
            const label = t(`nav.${item.key}`)
            return (
              <Tooltip key={item.key}>
                <TooltipTrigger asChild>
                  <button
                    onClick={() => onSelect(item.key)}
                    aria-label={label}
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
                <TooltipContent side="right">{label}</TooltipContent>
              </Tooltip>
            )
          })}
        </div>
      ))}
    </nav>
  )
}
