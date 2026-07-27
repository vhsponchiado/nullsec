import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Waypoints, Check, Info, Loader2 } from 'lucide-react'
import { api } from '@/lib/api'
import type { UpstreamProxyConfig, UpstreamProxyType } from '@/lib/types'

const EMPTY_CONFIG: UpstreamProxyConfig = { type: 'none', address: '', username: '', password: '' }

const TYPE_LABEL_KEY: Record<UpstreamProxyType, string> = {
  none: 'type_direct',
  socks5: 'type_socks5',
  http: 'type_http',
}

export function Settings() {
  const { t } = useTranslation('settings')
  const [config, setConfig] = useState<UpstreamProxyConfig>(EMPTY_CONFIG)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api
      .getUpstreamProxy()
      .then((cfg) => setConfig({ ...EMPTY_CONFIG, ...cfg }))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  async function handleSave() {
    setSaving(true)
    setError(null)
    setSaved(false)
    try {
      const result = await api.setUpstreamProxy(config)
      setConfig({ ...EMPTY_CONFIG, ...result })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const active = config.type !== 'none'

  return (
    <div className="max-w-2xl flex flex-col gap-6">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Waypoints size={20} className="text-primary" />
            <CardTitle>{t('upstream_proxy_title')}</CardTitle>
            {!loading && (
              <Badge variant={active ? 'secondary' : 'outline'} className="ml-auto">
                {active ? t(TYPE_LABEL_KEY[config.type]) : t('badge_direct')}
              </Badge>
            )}
          </div>
          <CardDescription>{t('upstream_description')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              {t('loading')}
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="settings-upstream-type">{t('type_label')}</Label>
                <Select
                  value={config.type}
                  onValueChange={(v) => setConfig({ ...config, type: v as UpstreamProxyType })}
                >
                  <SelectTrigger id="settings-upstream-type" size="lg" className="w-56">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.keys(TYPE_LABEL_KEY) as UpstreamProxyType[]).map((type) => (
                      <SelectItem key={type} value={type}>
                        {t(TYPE_LABEL_KEY[type])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {active && (
                <>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="settings-upstream-address">{t('address_label')}</Label>
                    <Input
                      id="settings-upstream-address"
                      size="lg"
                      className="font-mono"
                      value={config.address}
                      onChange={(e) => setConfig({ ...config, address: e.target.value })}
                      placeholder={
                        config.type === 'socks5'
                          ? t('address_placeholder_socks5')
                          : t('address_placeholder_http')
                      }
                    />
                  </div>
                  <div className="flex gap-3">
                    <div className="flex flex-col gap-1.5 flex-1">
                      <Label htmlFor="settings-upstream-username">{t('username_label')}</Label>
                      <Input
                        id="settings-upstream-username"
                        size="lg"
                        value={config.username}
                        onChange={(e) => setConfig({ ...config, username: e.target.value })}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5 flex-1">
                      <Label htmlFor="settings-upstream-password">{t('password_label')}</Label>
                      <Input
                        id="settings-upstream-password"
                        size="lg"
                        type="password"
                        value={config.password}
                        onChange={(e) => setConfig({ ...config, password: e.target.value })}
                      />
                    </div>
                  </div>
                </>
              )}

              <div className="flex items-center gap-3">
                <Button size="lg" onClick={handleSave} disabled={saving} className="w-fit">
                  {saving ? <Loader2 className="animate-spin" /> : saved ? <Check /> : null}
                  {saved ? t('saved') : t('save')}
                </Button>
                {error && <span className="text-sm text-destructive">{error}</span>}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('explainer_title')}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm text-muted-foreground">
          <p>{t('explainer_p1')}</p>
          <p>
            <b className="text-foreground">{t('explainer_p2_bold')}</b>
            {t('explainer_p2_rest')}
          </p>
          <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
            <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
            <p className="text-xs">{t('explainer_note')}</p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
