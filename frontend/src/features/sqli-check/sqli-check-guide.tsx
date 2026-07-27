import { ShieldAlert, AlertTriangle, CheckCircle2, Info } from 'lucide-react'
import { Trans, useTranslation } from 'react-i18next'
import { Step } from '@/components/guide-step'

const mono = <code className="font-mono" />
const bold = <b />
const boldFg = <b className="text-foreground" />

export function SqliGuide() {
  const { t } = useTranslation('sqli-check')

  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">{t('guide_how_to_use_title')}</h2>
        <p className="text-xs text-muted-foreground">{t('guide_intro')}</p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title={t('guide_step1_title')}>
          <Trans
            i18nKey="guide_step1_body"
            ns="sqli-check"
            components={[mono, mono, mono, mono, mono, mono]}
          />
        </Step>

        <Step n={2} title={t('guide_step2_title')}>
          {t('guide_step2_body')}
        </Step>

        <Step n={3} title={t('guide_step3_title')}>
          {t('guide_step3_body')}
        </Step>

        <Step n={4} title={t('guide_step4_title')}>
          {t('guide_step4_body')}
        </Step>
      </div>

      <div>
        <h2 className="font-semibold mb-2">{t('guide_icons_title')}</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <ShieldAlert className="size-4 text-destructive shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_critical_body" ns="sqli-check" components={[bold]} />
            </span>
          </div>
          <div className="flex items-center gap-2">
            <AlertTriangle className="size-4 text-warning shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_warning_body" ns="sqli-check" components={[bold]} />
            </span>
          </div>
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_info_body" ns="sqli-check" components={[bold]} />
            </span>
          </div>
        </div>
      </div>

      <div>
        <h2 className="font-semibold mb-2">{t('guide_payload_types_title')}</h2>
        <div className="flex flex-col gap-1.5 text-xs text-muted-foreground">
          <p>
            <Trans i18nKey="guide_payload_error_body" ns="sqli-check" components={[boldFg]} />
          </p>
          <p>
            <Trans
              i18nKey="guide_payload_boolean_body"
              ns="sqli-check"
              components={[boldFg, mono]}
            />
          </p>
          <p>
            <Trans i18nKey="guide_payload_time_body" ns="sqli-check" components={[boldFg, mono]} />
          </p>
        </div>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">{t('guide_footer_note')}</p>
      </div>
    </div>
  )
}
