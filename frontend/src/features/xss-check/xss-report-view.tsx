import { Badge } from '@/components/ui/badge'
import { ShieldAlert, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { groupBy } from '@/lib/utils'
import type { XssContext, XssReport, XssSeverity } from '@/lib/types'

export const CONTEXT_LABEL: Record<XssContext, string> = {
  html: 'HTML tag',
  attribute: 'Attribute break-out',
  js: 'Inline script break-out',
  url: 'URL / href',
}

const SEVERITY_ICON: Record<XssSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  info: <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />,
}

export function XssReportView({ report }: { report: XssReport }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex gap-2 items-center flex-wrap">
        <Badge variant="secondary">{report.summary.params_tested} parameters</Badge>
        <Badge variant="secondary">{report.summary.requests} requests</Badge>
        <Badge variant="secondary">{report.summary.vulnerable} signals found</Badge>
        {report.summary.vulnerable > 0 && (
          <Badge variant="outline" className="border-destructive text-destructive">
            possible XSS
          </Badge>
        )}
      </div>

      {report.summary.params_tested === 0 && (
        <div className="text-sm text-muted-foreground border rounded-lg p-4">
          No parameters found in the URL (query string) or the body. Add some to test, e.g.{' '}
          <code>?q=test</code>.
        </div>
      )}

      {groupBy(report.findings, (f) => `${f.location}:${f.parameter}`).map(([key, findings]) => {
        const [location, parameter] = key.split(':')
        const vulnerableCount = findings.filter((f) => f.vulnerable).length
        return (
          <div key={key} className="border rounded-lg min-w-0">
            <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
              <span className="font-mono">
                {parameter} <span className="text-muted-foreground">({location})</span>
              </span>
              {vulnerableCount > 0 ? (
                <Badge variant="outline" className="border-destructive text-destructive">
                  {vulnerableCount} signals
                </Badge>
              ) : (
                <Badge variant="secondary">No signal</Badge>
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
                        <Badge variant="secondary">{CONTEXT_LABEL[f.context]}</Badge>
                        <span className="font-mono text-xs text-muted-foreground">
                          {f.status || '—'} · {f.content_length}b
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
