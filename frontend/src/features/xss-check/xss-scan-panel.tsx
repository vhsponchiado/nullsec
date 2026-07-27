import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { EmptyState } from '@/components/empty-state'
import { Play, Square, Loader2, Bug } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import type { XssScanItemResult, XssScanResultEvent, XssScanStatus } from '@/lib/types'
import { XssScanItemView } from './xss-scan-item-view'

interface XssScanPanelProps {
  /** Loads a scan result's original request into Automatic mode for
   * focused manual follow-up on that one finding. */
  onOpenInChecker: (item: XssScanItemResult) => Promise<void>
}

/** XSS's "Scan (History)" mode: batch-runs the check across every request
 * already captured in Proxy History, instead of one URL at a time. */
export function XssScanPanel({ onOpenInChecker }: XssScanPanelProps) {
  const { t } = useTranslation('xss-check')
  const [scanHost, setScanHost] = useState('')
  const [scanId, setScanId] = useState<string | null>(null)
  const [scanStatus, setScanStatus] = useState<XssScanStatus | null>(null)
  const [scanTotal, setScanTotal] = useState<number | null>(null)
  const [scanResults, setScanResults] = useState<XssScanItemResult[]>([])
  const [scanStarting, setScanStarting] = useState(false)
  const [scanStartError, setScanStartError] = useState<string | null>(null)
  const [scanOnlyVulnerable, setScanOnlyVulnerable] = useState(false)
  const scanIdRef = useRef<string | null>(null)

  function mergeScanResults(incoming: XssScanItemResult[] | null | undefined) {
    setScanResults((prev) => {
      const known = new Set(prev.map((r) => r.history_id))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.history_id))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'xssscan_result') {
      const payload = event.payload as XssScanResultEvent
      if (payload.scan_id !== scanIdRef.current) return
      mergeScanResults([payload.result])
    } else if (event.type === 'xssscan_done' || event.type === 'xssscan_stopped') {
      const payload = event.payload as { scan_id: string }
      if (payload.scan_id !== scanIdRef.current) return
      setScanStatus(event.type === 'xssscan_done' ? 'done' : 'stopped')
    }
  })

  async function handleScanStart() {
    setScanStarting(true)
    setScanStartError(null)
    setScanResults([])
    setScanStatus(null)
    setScanTotal(null)
    try {
      const resp = await api.startXssScan({ host: scanHost.trim() || undefined })
      scanIdRef.current = resp.scan_id
      setScanId(resp.scan_id)
      setScanStatus('running')
      setScanTotal(resp.total)
      try {
        const snapshot = await api.getXssScan(resp.scan_id)
        mergeScanResults(snapshot.results)
        setScanStatus(snapshot.status)
      } catch {
        // WS updates will still arrive for anything after this point
      }
    } catch (err) {
      setScanStartError((err as Error).message)
    } finally {
      setScanStarting(false)
    }
  }

  async function handleScanStop() {
    if (!scanId) return
    try {
      await api.stopXssScan(scanId)
    } catch {
      // the scan may have already finished server-side, so there's nothing to do
    }
  }

  async function handleTest(item: XssScanItemResult) {
    try {
      await onOpenInChecker(item)
    } catch (err) {
      setScanStartError((err as Error).message)
    }
  }

  return (
    <>
      <div className="flex flex-col gap-4 border rounded-lg p-4">
        <h3 className="text-sm font-medium text-muted-foreground">{t('batch_scan_title')}</h3>
        <p className="text-xs text-muted-foreground">{t('batch_scan_description')}</p>

        <div className="flex gap-2 items-end flex-wrap">
          <div className="flex flex-col gap-1.5 flex-1 min-w-48">
            <Label htmlFor="xss-scan-host">{t('host_filter_label')}</Label>
            <Input
              id="xss-scan-host"
              size="lg"
              className="font-mono"
              value={scanHost}
              disabled={scanStatus === 'running'}
              onChange={(e) => setScanHost(e.target.value)}
              placeholder={t('host_filter_placeholder')}
            />
          </div>
          {scanStatus !== 'running' ? (
            <Button size="lg" onClick={handleScanStart} disabled={scanStarting} className="shrink-0">
              {scanStarting ? <Loader2 className="animate-spin" /> : <Play />}
              {t('start_scan_button')}
            </Button>
          ) : (
            <Button size="lg" variant="destructive" onClick={handleScanStop} className="shrink-0">
              <Square />
              {t('stop_button')}
            </Button>
          )}
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {scanStatus && (
            <Badge variant="outline">
              {scanStatus === 'running' && <Loader2 className="size-3 animate-spin" />}
              {scanStatus}
              {scanTotal !== null && ` · ${t('scan_done_suffix', { done: scanResults.length, total: scanTotal })}`}
            </Badge>
          )}
          {scanResults.length > 0 && (
            <Badge variant="secondary">
              {t('with_signals_badge', { count: scanResults.filter((r) => r.report.summary.vulnerable > 0).length })}
            </Badge>
          )}
        </div>

        {scanStartError && <div className="text-sm text-destructive">{scanStartError}</div>}
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-muted-foreground">{t('results_title')}</h3>
          {scanResults.length > 0 && (
            <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
              <Switch checked={scanOnlyVulnerable} onCheckedChange={setScanOnlyVulnerable} />
              {t('only_signals_toggle')}
            </label>
          )}
        </div>
        {scanResults.length === 0 ? (
          <EmptyState
            icon={Bug}
            title={t('empty_title')}
            description={t('empty_description')}
          />
        ) : (
          <div className="flex flex-col gap-2">
            {[...scanResults]
              .sort((a, b) => b.report.summary.vulnerable - a.report.summary.vulnerable)
              .filter((item) => !scanOnlyVulnerable || item.report.summary.vulnerable > 0)
              .map((item) => (
                <XssScanItemView key={item.history_id} item={item} onTest={handleTest} />
              ))}
            {scanOnlyVulnerable && scanResults.every((r) => r.report.summary.vulnerable === 0) && (
              <p className="text-xs text-muted-foreground">{t('no_signals_yet')}</p>
            )}
          </div>
        )}
      </div>
    </>
  )
}
