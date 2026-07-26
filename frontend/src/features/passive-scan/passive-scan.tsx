import { useMemo, useState } from 'react'
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
  const [open, setOpen] = useState(false)
  return (
    <div className="border rounded-lg min-w-0">
      <div className="flex items-start gap-3 px-4 py-3 min-w-0">
        <div className="pt-0.5">{SEVERITY_ICON[group.severity]}</div>
        <div className="min-w-0 flex-1 flex flex-col gap-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium">{group.title}</span>
            <Badge variant="secondary">{group.host}</Badge>
            <Badge variant="outline">{group.urls.length} page{group.urls.length === 1 ? '' : 's'}</Badge>
          </div>
          <p className="text-xs text-muted-foreground">{group.detail}</p>
          <Button size="sm" variant="ghost" className="w-fit -ml-2 text-muted-foreground" onClick={() => setOpen((v) => !v)}>
            {open ? <ChevronUp /> : <ChevronDown />}
            {open ? 'Hide' : 'Show'} affected URLs
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
          <h3 className="text-sm font-medium text-muted-foreground">Scan captured traffic</h3>
          <p className="text-xs text-muted-foreground">
            Analyzes everything already sitting in Proxy History — no new requests are made.
          </p>

          <div className="flex gap-2 items-end">
            <div className="flex flex-col gap-1.5 flex-1">
              <Label htmlFor="passive-scan-host">Filter by host (optional)</Label>
              <Input
                id="passive-scan-host"
                size="lg"
                className="font-mono"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="example.com"
              />
            </div>
            <Button size="lg" onClick={handleScan} disabled={loading} className="shrink-0">
              {loading ? <Loader2 className="animate-spin" /> : <ScanEye />}
              Scan
            </Button>
          </div>

          {error && <div className="text-sm text-destructive">{error}</div>}

          {report && (
            <div className="flex gap-2 items-center flex-wrap">
              <Badge variant="secondary">{report.summary.requests_scanned} requests scanned</Badge>
              <Badge variant="secondary">{report.summary.findings} findings</Badge>
              {report.summary.critical > 0 && (
                <Badge variant="outline" className="border-destructive text-destructive">
                  {report.summary.critical} critical
                </Badge>
              )}
              {report.summary.warning > 0 && (
                <Badge variant="outline" className="border-warning text-warning">
                  {report.summary.warning} warning
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
                title="No issues found"
                description={
                  report.summary.requests_scanned === 0
                    ? 'Proxy History is empty — capture some traffic first.'
                    : 'Nothing flagged across the scanned requests.'
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
