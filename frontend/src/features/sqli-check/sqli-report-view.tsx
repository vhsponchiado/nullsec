import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { ShieldAlert, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { groupBy } from '@/lib/utils'
import type { SqliReport, SqliSeverity } from '@/lib/types'

const SEVERITY_ICON: Record<SqliSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  warning: <AlertTriangle className="size-4 text-warning shrink-0" />,
  info: <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />,
}

export function SqliReportView({ report }: { report: SqliReport }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <div className="flex gap-2 items-center flex-wrap">
          <Badge variant="secondary">{report.summary.params_tested} parameters</Badge>
          <Badge variant="secondary">{report.summary.requests} requests</Badge>
          <Badge variant="secondary">{report.summary.vulnerable} signals found</Badge>
          {report.summary.critical > 0 && (
            <Badge variant="outline" className="border-destructive text-destructive">
              {report.summary.critical} critical
            </Badge>
          )}
        </div>
        <span className="text-xs text-muted-foreground">
          baseline: status {report.baseline.status || '—'}, {report.baseline.duration_ms}
          ms, {report.baseline.content_length} bytes
        </span>
      </div>

      {report.summary.params_tested === 0 && (
        <div className="text-sm text-muted-foreground border rounded-lg p-4">
          No parameters found in the URL (query string) or the body. Add some to test, e.g.{' '}
          <code>?id=1</code>.
        </div>
      )}

      {report.boolean_pairs && report.boolean_pairs.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h3 className="text-sm font-medium">True/false test</h3>
            <p className="text-xs text-muted-foreground">
              We send a condition that should always be <strong>true</strong> and another that's
              always <strong>false</strong>. If the site responds differently to each one, that's
              a strong signal of SQL injection. If it responds the same to both, there's no signal
              here.
            </p>
          </div>
          {groupBy(report.boolean_pairs, (p) => `${p.location}:${p.parameter}`).map(([key, pairs]) => {
            const [location, parameter] = key.split(':')
            const vulnerableCount = pairs.filter((p) => p.vulnerable).length
            const isVulnerable = vulnerableCount > 0
            return (
              <div key={key} className="border rounded-lg min-w-0">
                <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
                  <span className="font-mono">
                    {parameter} <span className="text-muted-foreground">({location})</span>
                  </span>
                  {isVulnerable ? (
                    <Badge variant="outline" className="border-destructive text-destructive">
                      Possible SQL injection
                    </Badge>
                  ) : (
                    <Badge variant="secondary">No signal</Badge>
                  )}
                </div>
                <div className="divide-y">
                  {pairs.map((p, i) => (
                    <div key={i} className="flex items-start gap-3 px-4 py-3 min-w-0">
                      <div className="pt-0.5">
                        {p.error ? (
                          <AlertTriangle className="size-4 text-muted-foreground shrink-0" />
                        ) : (
                          SEVERITY_ICON[p.severity]
                        )}
                      </div>
                      <div className="min-w-0 flex-1 flex flex-col gap-1">
                        <p className="text-xs break-words">{p.error || p.note}</p>
                        <p className="text-[11px] text-muted-foreground font-mono break-all">
                          true → status {p.true_status || '—'} ({p.true_content_length} bytes) ·
                          false → status {p.false_status || '—'} ({p.false_content_length} bytes)
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}

      <Separator />

      <div className="flex flex-col gap-3">
        <h3 className="text-sm font-medium">Database error and response delay</h3>
        <p className="text-xs text-muted-foreground">
          Each row below only shows up if some payload caused a visible database error in the
          response, or took much longer than normal to respond (time-based "blind" SQLi signal).
        </p>
      </div>

      {groupBy(report.findings, (f) => `${f.location}:${f.parameter}`).map(([key, findings]) => {
        const [location, parameter] = key.split(':')
        const vulnerableCount = findings.filter((f) => f.vulnerable).length
        return (
          <div key={key} className="border rounded-lg min-w-0">
            <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
              <span className="font-mono">
                {parameter} <span className="text-muted-foreground">({location})</span>
              </span>
              {vulnerableCount > 0 && (
                <Badge variant="outline" className="border-destructive text-destructive">
                  {vulnerableCount} signals
                </Badge>
              )}
            </div>
            <div className="divide-y">
              {findings
                .filter((f) => f.vulnerable || f.error)
                .map((f, i) => (
                  <div key={i} className="flex items-start gap-3 px-4 py-3 min-w-0">
                    <div className="pt-0.5">
                      {f.error ? (
                        <AlertTriangle className="size-4 text-muted-foreground shrink-0" />
                      ) : (
                        SEVERITY_ICON[f.severity]
                      )}
                    </div>
                    <div className="min-w-0 flex-1 flex flex-col gap-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-mono text-xs font-medium break-all">{f.payload}</span>
                        <Badge variant="secondary">{f.payload_type}</Badge>
                        <span className="font-mono text-xs text-muted-foreground">
                          {f.status || '—'} · {f.duration_ms}ms · {f.content_length}b
                        </span>
                      </div>
                      <p className="text-xs text-muted-foreground break-words">{f.error || f.note}</p>
                    </div>
                  </div>
                ))}
              {findings.every((f) => !f.vulnerable && !f.error) && (
                <div className="px-4 py-3 text-xs text-muted-foreground">
                  No signal across {findings.length} payloads tested.
                </div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
