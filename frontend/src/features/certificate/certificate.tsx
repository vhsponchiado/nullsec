import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ShieldCheck, Download, RefreshCw } from 'lucide-react'
import { api, caCertUrl } from '@/lib/api'

const mono = <code />

export function Certificate() {
  const { t } = useTranslation('certificate')
  const [regenerating, setRegenerating] = useState(false)

  async function handleRegenerate() {
    if (!confirm(t('regenerate_confirm'))) return
    setRegenerating(true)
    try {
      await api.regenerateCA()
    } finally {
      setRegenerating(false)
    }
  }

  return (
    <div className="max-w-2xl flex flex-col gap-6">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <ShieldCheck size={20} className="text-primary" />
            <CardTitle>{t('ca_title')}</CardTitle>
          </div>
          <CardDescription>{t('ca_description')}</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-3">
          <Button size="lg" asChild>
            <a href={caCertUrl} download="nullsec-ca-cert.pem">
              <Download />
              {t('download_certificate')}
            </a>
          </Button>
          <Button size="lg" variant="outline" onClick={handleRegenerate} disabled={regenerating}>
            <RefreshCw className={regenerating ? 'animate-spin' : ''} />
            {t('regenerate_ca')}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('auto_install_title')}</CardTitle>
          <CardDescription>
            <Trans
              i18nKey="auto_install_description"
              ns="certificate"
              values={{ pkg: 'smallstep/truststore' }}
              components={[mono]}
            />
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted rounded-md p-4 text-sm font-mono overflow-x-auto">
            go run ./cmd/server cert install
          </pre>
          <p className="text-sm text-muted-foreground mt-3">
            <Trans i18nKey="auto_install_note" ns="certificate" components={[mono]} />
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('no_install_title')}</CardTitle>
          <CardDescription>
            <Trans i18nKey="no_install_description" ns="certificate" components={[mono]} />
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted rounded-md p-4 text-sm font-mono overflow-x-auto">
            go run ./cmd/server chrome
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
