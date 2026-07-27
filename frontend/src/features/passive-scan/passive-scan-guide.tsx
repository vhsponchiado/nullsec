import { ShieldAlert, AlertTriangle, Info } from 'lucide-react'
import { Trans, useTranslation } from 'react-i18next'

const mono = <span className="font-mono" />
const bold = <b className="text-foreground" />
const boldPlain = <b />

export function PassiveScanGuide() {
  const { t } = useTranslation('passive-scan')

  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">{t('guide_how_to_use_title')}</h2>
        <p className="text-xs text-muted-foreground">{t('guide_intro')}</p>
      </div>

      <div>
        <h2 className="font-semibold mb-2">{t('guide_what_it_checks_title')}</h2>
        <div className="flex flex-col gap-3 text-xs text-muted-foreground">
          <p>
            <Trans i18nKey="guide_check_missing_headers" ns="passive-scan" components={[bold]} />
          </p>
          <p>
            <Trans i18nKey="guide_check_cookie_flags" ns="passive-scan" components={[bold, mono, mono, mono, mono]} />
          </p>
          <p>
            <Trans i18nKey="guide_check_sensitive_data" ns="passive-scan" components={[bold, mono, mono, mono]} />
          </p>
          <p>
            <Trans i18nKey="guide_check_server_version" ns="passive-scan" components={[bold, mono, mono]} />
          </p>
          <p>
            <Trans i18nKey="guide_check_permissive_cors" ns="passive-scan" components={[bold, mono, mono]} />
          </p>
          <p>
            <Trans i18nKey="guide_check_credentials_http" ns="passive-scan" components={[bold]} />
          </p>
        </div>
      </div>

      <div>
        <h2 className="font-semibold mb-2">{t('guide_what_each_icon_means_title')}</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <ShieldAlert className="size-4 text-destructive shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_critical" ns="passive-scan" components={[boldPlain]} />
            </span>
          </div>
          <div className="flex items-center gap-2">
            <AlertTriangle className="size-4 text-warning shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_warning" ns="passive-scan" components={[boldPlain]} />
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Info className="size-4 text-muted-foreground shrink-0" />
            <span className="text-xs">
              <Trans i18nKey="guide_icon_info" ns="passive-scan" components={[boldPlain]} />
            </span>
          </div>
        </div>
      </div>

      <p className="text-xs text-muted-foreground">{t('guide_footer_note')}</p>
    </div>
  )
}
