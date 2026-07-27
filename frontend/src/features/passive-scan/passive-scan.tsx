import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { EmptyState } from '@/components/empty-state'
import { ShieldAlert, AlertTriangle, Info, Loader2, ScanEye, ChevronDown, ChevronUp } from 'lucide-react'
import { api } from '@/lib/api'
import type { PassiveFinding, PassiveReport, PassiveSeverity } from '@/lib/types'
import { PassiveScanGuide } from './passive-scan-guide'

const SEVERITY_ICON: Record<PassiveSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  warning: <AlertTriangle className="size-4 text-warning shrink-0" />,
  info: <Info className="size-4 text-muted-foreground shrink-0" />,
}

const SEVERITY_RANK: Record<PassiveSeverity, number> = { critical: 0, warning: 1, info: 2 }

interface GroupedFinding {
  key: string
  host: string
  title: string
  severity: PassiveSeverity
  detail: string
  urls: string[]
}

function groupFindings(findings: PassiveFinding[]): GroupedFinding[] {
  const order: string[] = []
  const groups = new Map<string, GroupedFinding>()
  for (const f of findings) {
    const key = `${f.host}::${f.title}`
    if (!groups.has(key)) {
      groups.set(key, { key, host: f.host, title: f.title, severity: f.severity, detail: f.detail, urls: [] })
      order.push(key)
    }
    groups.get(key)!.urls.push(f.url)
  }
  return order
    .map((k) => groups.get(k)!)
    .sort((a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || a.host.localeCompare(b.host))
}

function GroupCard({ group }: { group: GroupedFinding }) {
  const { t } = useTranslation('passive-scan')
  const [open, setOpen] = useState(false)
  return (
    <div className="border rounded-lg min-w-0">
      <div className="flex items-start gap-3 px-4 py-3 min-w-0">
        <div className="pt-0.5">{SEVERITY_ICON[group.severity]}</div>
        <div className="min-w-0 flex-1 flex flex-col gap-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium">{group.title}</span>
            <Badge variant="secondary">{group.host}</Badge>
            <Badge variant="outline">{t('page_count', { count: group.urls.length })}</Badge>
          </div>
          <p className="text-xs text-muted-foreground">{group.detail}</p>
          <Button size="sm" variant="ghost" className="w-fit -ml-2 text-muted-foreground" onClick={() => setOpen((v) => !v)}>
            {open ? <ChevronUp /> : <ChevronDown />}
            {t(open ? 'hide_affected_urls' : 'show_affected_urls')}
          </Button>
          {open && (
            <div className="flex flex-col gap-1 mt-1 max-h-40 overflow-auto">
              {group.urls.map((u, i) => (
                <span key={i} className="font-mono text-[11px] text-muted-foreground break-all">
                  {u}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export function PassiveScan() {
  const { t } = useTranslation('passive-scan')
  const [host, setHost] = useState('')
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<PassiveReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function handleScan() {
    setLoading(true)
    setError(null)
    try {
      const result = await api.passiveScan({ host: host.trim() || undefined })
      setReport(result)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const grouped = useMemo(() => (report ? groupFindings(report.findings) : []), [report])

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">{t('scan_title')}</h3>
          <p className="text-xs text-muted-foreground">
            {t('scan_description')}
          </p>

          <div className="flex gap-2 items-end">
            <div className="flex flex-col gap-1.5 flex-1">
              <Label htmlFor="passive-scan-host">{t('host_filter_label')}</Label>
              <Input
                id="passive-scan-host"
                size="lg"
                className="font-mono"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder={t('host_placeholder')}
              />
            </div>
            <Button size="lg" onClick={handleScan} disabled={loading} className="shrink-0">
              {loading ? <Loader2 className="animate-spin" /> : <ScanEye />}
              {t('scan_button')}
            </Button>
          </div>

          {error && <div className="text-sm text-destructive">{error}</div>}

          {report && (
            <div className="flex gap-2 items-center flex-wrap">
              <Badge variant="secondary">{t('requests_scanned', { count: report.summary.requests_scanned })}</Badge>
              <Badge variant="secondary">{t('findings_count', { count: report.summary.findings })}</Badge>
              {report.summary.critical > 0 && (
                <Badge variant="outline" className="border-destructive text-destructive">
                  {t('critical_count', { count: report.summary.critical })}
                </Badge>
              )}
              {report.summary.warning > 0 && (
                <Badge variant="outline" className="border-warning text-warning">
                  {t('warning_count', { count: report.summary.warning })}
                </Badge>
              )}
            </div>
          )}
        </div>

        {report && (
          <div className="flex flex-col gap-3">
            {grouped.length === 0 ? (
              <EmptyState
                icon={ScanEye}
                title={t('empty_title')}
                description={
                  report.summary.requests_scanned === 0
                    ? t('empty_description_no_traffic')
                    : t('empty_description_no_findings')
                }
              />
            ) : (
              grouped.map((g) => <GroupCard key={g.key} group={g} />)
            )}
          </div>
        )}
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <PassiveScanGuide />
      </aside>
    </div>
  )
}
