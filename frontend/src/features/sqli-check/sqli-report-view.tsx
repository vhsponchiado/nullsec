import { Trans, useTranslation } from 'react-i18next'
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

const code = <code />
const strong = <strong />

export function SqliReportView({ report }: { report: SqliReport }) {
  const { t } = useTranslation('sqli-check')
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <div className="flex gap-2 items-center flex-wrap">
          <Badge variant="secondary">{t('params_badge', { count: report.summary.params_tested })}</Badge>
          <Badge variant="secondary">{t('requests_badge', { count: report.summary.requests })}</Badge>
          <Badge variant="secondary">{t('signals_found_badge', { count: report.summary.vulnerable })}</Badge>
          {report.summary.critical > 0 && (
            <Badge variant="outline" className="border-destructive text-destructive">
              {t('critical_badge', { count: report.summary.critical })}
            </Badge>
          )}
        </div>
        <span className="text-xs text-muted-foreground">
          {t('baseline_summary', {
            status: report.baseline.status || t('not_available'),
            duration: report.baseline.duration_ms,
            length: report.baseline.content_length,
          })}
        </span>
      </div>

      {report.summary.params_tested === 0 && (
        <div className="text-sm text-muted-foreground border rounded-lg p-4">
          <Trans i18nKey="no_params_body" ns="sqli-check" components={[code]} />
        </div>
      )}

      {report.boolean_pairs && report.boolean_pairs.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h3 className="text-sm font-medium">{t('boolean_test_title')}</h3>
            <p className="text-xs text-muted-foreground">
              <Trans i18nKey="boolean_test_body" ns="sqli-check" components={[strong, strong]} />
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
                      {t('possible_sqli')}
                    </Badge>
                  ) : (
                    <Badge variant="secondary">{t('no_signal')}</Badge>
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
                          {t('boolean_pair_summary', {
                            trueStatus: p.true_status || t('not_available'),
                            trueLength: p.true_content_length,
                            falseStatus: p.false_status || t('not_available'),
                            falseLength: p.false_content_length,
                          })}
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
        <h3 className="text-sm font-medium">{t('error_delay_title')}</h3>
        <p className="text-xs text-muted-foreground">{t('error_delay_body')}</p>
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
                  {t('signals_count_badge', { count: vulnerableCount })}
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
                          {t('finding_status_summary', {
                            status: f.status || t('not_available'),
                            duration: f.duration_ms,
                            length: f.content_length,
                          })}
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
