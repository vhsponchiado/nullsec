import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Terminal, UserRound, Lock } from 'lucide-react'

const TOOL_NAV_KEYS = [
  'intercept',
  'repeater',
  'intruder',
  'spider',
  'passive-scan',
  'port-scanner',
  'subdomain-finder',
  'dir-finder',
  'wordpress-check',
  'sqli-check',
  'xss-check',
  'decoder',
] as const

export function About() {
  const { t } = useTranslation('about')
  const { t: tCommon } = useTranslation('common')

  return (
    <div className="max-w-2xl flex flex-col gap-6">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Terminal size={20} className="text-primary" />
            <CardTitle>{t('project_title')}</CardTitle>
          </div>
          <CardDescription>{t('project_description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-1.5">
            {TOOL_NAV_KEYS.map((key) => (
              <Badge key={key} variant="secondary" className="text-xs">
                {tCommon(`nav.${key}`)}
              </Badge>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <UserRound size={20} className="text-primary" />
            <CardTitle>{t('credits_title')}</CardTitle>
          </div>
          <CardDescription>{t('credits_description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-lg font-semibold">{t('developer_name')}</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Lock size={20} className="text-primary" />
            <CardTitle>{t('private_title')}</CardTitle>
          </div>
          <CardDescription>{t('private_description')}</CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
