import { Info } from 'lucide-react'
import { Trans, useTranslation } from 'react-i18next'
import { Step } from '@/components/guide-step'

const mono = <span className="font-mono" />
const bold = <b className="text-foreground" />

export function IntruderGuide() {
  const { t } = useTranslation('intruder')

  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">{t('guide_how_to_use_title')}</h2>
        <p className="text-xs text-muted-foreground">{t('guide_intro')}</p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title={t('guide_step1_title')}>
          <Trans i18nKey="guide_step1_body" ns="intruder" components={[mono, mono]} />
        </Step>

        <Step n={2} title={t('guide_step2_title')}>
          <Trans i18nKey="guide_step2_body" ns="intruder" components={[bold, bold, bold, bold]} />
        </Step>

        <Step n={3} title={t('guide_step3_title')}>
          <Trans i18nKey="guide_step3_body" ns="intruder" components={[bold, bold, bold]} />
        </Step>

        <Step n={4} title={t('guide_step4_title')}>
          <Trans i18nKey="guide_step4_body" ns="intruder" components={[mono]} />
        </Step>

        <Step n={5} title={t('guide_step5_title')}>
          {t('guide_step5_body')}
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">{t('guide_footer_note')}</p>
      </div>
    </div>
  )
}
