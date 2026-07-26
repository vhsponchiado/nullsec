import { useState } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { AlertTriangle, ShieldAlert, CheckCircle2, Loader2 } from 'lucide-react'
import { api } from '@/lib/api'
import { groupBy } from '@/lib/utils'
import type { WordPressCheckReport, WPCheckSeverity } from '@/lib/types'

const SEVERITY_ICON: Record<WPCheckSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  warning: <AlertTriangle className="size-4 text-warning shrink-0" />,
  info: <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />,
}

export function WordPressCheck() {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<WordPressCheckReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function handleScan() {
    if (!url.trim()) return
    setLoading(true)
    setError(null)
    try {
      const result = await api.wordpressCheck(url.trim())
      setReport(result)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col gap-4 h-full min-w-0">
      <div className="flex flex-wrap gap-2 items-center max-w-2xl">
        <Input
          size="lg"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleScan()}
          placeholder="https://authorized-target.com"
          className="font-mono flex-1 min-w-48"
        />
        <Button size="lg" onClick={handleScan} disabled={loading}>
          {loading && <Loader2 className="animate-spin" />}
          Scan
        </Button>
      </div>

      <p className="text-xs text-muted-foreground max-w-2xl">
        Only makes read-only GET requests (fingerprinting) against known WordPress endpoints. Use
        only on targets you're authorized to test.
      </p>

      {error && <div className="text-sm text-destructive">{error}</div>}

      {report && (
        <div className="flex-1 min-h-0 overflow-auto flex flex-col gap-5 pr-1">
          <div className="flex gap-2 items-center flex-wrap">
            <Badge variant="secondary">{report.summary.checked} checked</Badge>
            <Badge variant="secondary">{report.summary.found} found</Badge>
            {report.summary.critical > 0 && (
              <Badge variant="outline" className="border-destructive text-destructive">
                {report.summary.critical} critical
              </Badge>
            )}
          </div>

          {groupBy(report.results, (r) => r.category).map(([category, results]) => (
            <div key={category} className="border rounded-lg min-w-0">
              <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
                {category}
              </div>
              <div className="divide-y">
                {results.map((r) => (
                  <div
                    key={r.path}
                    className={`flex items-start gap-3 px-4 py-3 min-w-0 ${r.found ? '' : 'opacity-50'}`}
                  >
                    <div className="pt-0.5">{r.found && SEVERITY_ICON[r.severity]}</div>

                    <div className="min-w-0 flex-1 flex flex-col gap-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-mono text-xs font-medium break-all">{r.path}</span>
                        <span className="font-mono text-xs text-muted-foreground">
                          {r.error ? 'error' : r.status || '—'}
                        </span>
                        {r.found ? (
                          <Badge
                            variant={r.severity === 'critical' ? 'outline' : 'secondary'}
                            className={
                              r.severity === 'critical' ? 'border-destructive text-destructive' : ''
                            }
                          >
                            Found
                          </Badge>
                        ) : (
                          <Badge variant="outline">Not found</Badge>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground break-words">
                        {r.error || r.note || r.description}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
