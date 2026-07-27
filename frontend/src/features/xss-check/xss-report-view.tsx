import { Trans, useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { ShieldAlert, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { groupBy } from '@/lib/utils'
import type { XssContext, XssReport, XssSeverity } from '@/lib/types'

export const CONTEXT_LABEL_KEY: Record<XssContext, string> = {
  html: 'context_html',
  attribute: 'context_attribute',
  js: 'context_js',
  url: 'context_url',
}

const SEVERITY_ICON: Record<XssSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  info: <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />,
}

const code = <code />

export function XssReportView({ report }: { report: XssReport }) {
  const { t } = useTranslation('xss-check')

  return (
    <div className="flex flex-col gap-6">
      <div className="flex gap-2 items-center flex-wrap">
        <Badge variant="secondary">{t('params_tested_badge', { count: report.summary.params_tested })}</Badge>
        <Badge variant="secondary">{t('requests_badge', { count: report.summary.requests })}</Badge>
        <Badge variant="secondary">{t('signals_found_badge', { count: report.summary.vulnerable })}</Badge>
        {report.summary.vulnerable > 0 && (
          <Badge variant="outline" className="border-destructive text-destructive">
            {t('possible_xss_badge')}
          </Badge>
        )}
      </div>

      {report.summary.params_tested === 0 && (
        <div className="text-sm text-muted-foreground border rounded-lg p-4">
          <Trans i18nKey="no_params_found" ns="xss-check" components={[code]} />
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
                  {t('param_signals_badge', { count: vulnerableCount })}
                </Badge>
              ) : (
                <Badge variant="secondary">{t('no_signal_badge')}</Badge>
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
                        <Badge variant="secondary">{t(CONTEXT_LABEL_KEY[f.context])}</Badge>
                        <span className="font-mono text-xs text-muted-foreground">
                          {f.status || t('status_na')} · {f.content_length}b
                        </span>
                      </div>
                      <p className="text-xs text-muted-foreground break-words">{f.error || f.note}</p>
                    </div>
                  </div>
                ))}
              {findings.every((f) => !f.vulnerable && !f.error) && (
                <div className="px-4 py-3 text-xs text-muted-foreground">
                  {t('no_signal_across', { count: findings.length })}
                </div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
