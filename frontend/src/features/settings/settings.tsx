import { useEffect, useState } from 'react'
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

const TYPE_LABEL: Record<UpstreamProxyType, string> = {
  none: 'None (direct)',
  socks5: 'SOCKS5',
  http: 'HTTP(S) proxy',
}

export function Settings() {
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
            <CardTitle>Upstream proxy</CardTitle>
            {!loading && (
              <Badge variant={active ? 'secondary' : 'outline'} className="ml-auto">
                {active ? TYPE_LABEL[config.type] : 'Direct'}
              </Badge>
            )}
          </div>
          <CardDescription>
            Route outbound traffic from Repeater, Intruder, Port Scanner and the Subdomain
            Finder's crt.sh lookup through a proxy you control — a local Tor/VPN SOCKS listener,
            or an HTTP(S) forward proxy (e.g. a client-provided jump box) — instead of dialing
            targets directly from this machine.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Loading…
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="settings-upstream-type">Type</Label>
                <Select
                  value={config.type}
                  onValueChange={(v) => setConfig({ ...config, type: v as UpstreamProxyType })}
                >
                  <SelectTrigger id="settings-upstream-type" size="lg" className="w-56">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.keys(TYPE_LABEL) as UpstreamProxyType[]).map((t) => (
                      <SelectItem key={t} value={t}>
                        {TYPE_LABEL[t]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {active && (
                <>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="settings-upstream-address">Address</Label>
                    <Input
                      id="settings-upstream-address"
                      size="lg"
                      className="font-mono"
                      value={config.address}
                      onChange={(e) => setConfig({ ...config, address: e.target.value })}
                      placeholder={config.type === 'socks5' ? '127.0.0.1:9050' : 'proxy.example.com:8080'}
                    />
                  </div>
                  <div className="flex gap-3">
                    <div className="flex flex-col gap-1.5 flex-1">
                      <Label htmlFor="settings-upstream-username">Username (optional)</Label>
                      <Input
                        id="settings-upstream-username"
                        size="lg"
                        value={config.username}
                        onChange={(e) => setConfig({ ...config, username: e.target.value })}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5 flex-1">
                      <Label htmlFor="settings-upstream-password">Password (optional)</Label>
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
                  {saved ? 'Saved' : 'Save'}
                </Button>
                {error && <span className="text-sm text-destructive">{error}</span>}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>What this does and doesn't do</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm text-muted-foreground">
          <p>
            Once set, every request Repeater and Intruder send, every port the Port Scanner
            probes, and the Subdomain Finder's crt.sh lookup all go through this proxy instead of
            leaving directly from this machine.
          </p>
          <p>
            <b className="text-foreground">DNS is the exception</b> — the Subdomain Finder's
            brute-force step resolves names with the system resolver directly; SOCKS5/HTTP
            proxying doesn't cover DNS the same way.
          </p>
          <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
            <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
            <p className="text-xs">
              This changes which IP your traffic originates from — it doesn't change whether using
              these tools against a given target is authorized. Only route engagement traffic
              through infrastructure you're entitled to use.
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
