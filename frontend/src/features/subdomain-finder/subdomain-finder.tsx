import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { EmptyState } from '@/components/empty-state'
import { Play, Square, Loader2, ArrowUpDown, Globe } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import { useSortedResults } from '@/lib/use-sorted-results'
import type {
  SubdomainResult,
  SubdomainResultEvent,
  SubdomainStatus,
  SubdomainTotalEvent,
} from '@/lib/types'
import { SubdomainFinderGuide } from './subdomain-finder-guide'

type SortKey = 'subdomain' | 'duration_ms'

type ScanSpeed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<ScanSpeed, { concurrency: number; timeoutMs: number }> = {
  fast: { concurrency: 100, timeoutMs: 2000 },
  normal: { concurrency: 50, timeoutMs: 3000 },
  cautious: { concurrency: 10, timeoutMs: 5000 },
}

export function SubdomainFinder() {
  const { t } = useTranslation('subdomain-finder')
  const SOURCE_LABEL: Record<string, string> = {
    root: t('source_root'),
    crtsh: t('source_crtsh'),
    bruteforce: t('source_bruteforce'),
  }
  const [domain, setDomain] = useState('')
  const [useCRTSH, setUseCRTSH] = useState(true)
  const [wordlistSize, setWordlistSize] = useState('1000')
  const [wordlistTotal, setWordlistTotal] = useState(5000)
  const [speed, setSpeed] = useState<ScanSpeed>('normal')
  const [showUnresolved, setShowUnresolved] = useState(false)

  useEffect(() => {
    api.subdomainWordlistSize().then((r) => setWordlistTotal(r.size)).catch(() => {})
  }, [])

  const [scanId, setScanId] = useState<string | null>(null)
  const [status, setStatus] = useState<SubdomainStatus | null>(null)
  const [results, setResults] = useState<SubdomainResult[]>([])
  const [total, setTotal] = useState<number | null>(null)
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [warning, setWarning] = useState<string | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('subdomain')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const scanIdRef = useRef<string | null>(null)

  function mergeResults(incoming: SubdomainResult[] | null | undefined) {
    setResults((prev) => {
      const known = new Set(prev.map((r) => r.subdomain))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.subdomain))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'subdomain_result') {
      const payload = event.payload as SubdomainResultEvent
      if (payload.scan_id !== scanIdRef.current) return
      mergeResults([payload.result])
    } else if (event.type === 'subdomain_total') {
      const payload = event.payload as SubdomainTotalEvent
      if (payload.scan_id !== scanIdRef.current) return
      setTotal(payload.total)
    } else if (event.type === 'subdomain_done' || event.type === 'subdomain_stopped') {
      const payload = event.payload as { scan_id: string }
      if (payload.scan_id !== scanIdRef.current) return
      setStatus(event.type === 'subdomain_done' ? 'done' : 'stopped')
    }
  })

  async function handleStart() {
    if (!domain.trim()) {
      setStartError(t('domain_required'))
      return
    }
    if (!useCRTSH && wordlistSize === '0') {
      setStartError(t('crtsh_or_bruteforce_required'))
      return
    }

    setStarting(true)
    setStartError(null)
    setWarning(null)
    setResults([])
    setStatus(null)
    setTotal(null)
    try {
      const preset = SPEED_PRESETS[speed]
      const resp = await api.startSubdomainScan({
        domain: domain.trim(),
        use_crtsh: useCRTSH,
        wordlist_size: parseInt(wordlistSize, 10),
        concurrency: preset.concurrency,
        timeout_ms: preset.timeoutMs,
      })
      scanIdRef.current = resp.scan_id
      setScanId(resp.scan_id)
      setStatus('running')
      setTotal(resp.total)
      try {
        const snapshot = await api.getSubdomainScan(resp.scan_id)
        mergeResults(snapshot.results)
        setStatus(snapshot.status)
        setTotal(snapshot.total || resp.total)
        if (snapshot.warning) setWarning(snapshot.warning)
      } catch {
        // WS updates will still arrive for anything after this point
      }
    } catch (err) {
      setStartError((err as Error).message)
    } finally {
      setStarting(false)
    }
  }

  async function handleStop() {
    if (!scanId) return
    try {
      await api.stopSubdomainScan(scanId)
    } catch {
      // the scan may have already finished server-side, so there is nothing to do
    }
  }

  function toggleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  const visible = showUnresolved ? results : results.filter((r) => r.resolved)

  const compare = useCallback(
    (a: SubdomainResult, b: SubdomainResult) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (av < bv) return sortDir === 'asc' ? -1 : 1
      if (av > bv) return sortDir === 'asc' ? 1 : -1
      return 0
    },
    [sortKey, sortDir],
  )
  const sorted = useSortedResults(visible, compare)

  const foundCount = results.filter((r) => r.resolved).length
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">{t('target_section_title')}</h3>

          <div className="flex gap-2 items-end flex-wrap">
            <div className="flex flex-col gap-1.5 flex-1 min-w-48">
              <Label htmlFor="subdomain-domain">{t('domain_label')}</Label>
              <Input
                id="subdomain-domain"
                size="lg"
                className="font-mono"
                value={domain}
                disabled={running}
                onChange={(e) => setDomain(e.target.value)}
                placeholder={t('domain_placeholder')}
              />
            </div>
            {!running ? (
              <Button size="lg" onClick={handleStart} disabled={starting} className="shrink-0">
                {starting ? <Loader2 className="animate-spin" /> : <Play />}
                {t('start_scan')}
              </Button>
            ) : (
              <Button size="lg" variant="destructive" onClick={handleStop} className="shrink-0">
                <Square />
                {t('stop')}
              </Button>
            )}
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <Switch id="subdomain-use-crtsh" checked={useCRTSH} onCheckedChange={setUseCRTSH} disabled={running} />
              <Label htmlFor="subdomain-use-crtsh" className="text-sm">{t('crtsh_label')}</Label>
            </div>
            <div className="flex items-center gap-2">
              <Label htmlFor="subdomain-wordlist-size" className="text-sm text-muted-foreground">{t('dns_bruteforce_label')}</Label>
              <Select value={wordlistSize} onValueChange={setWordlistSize} disabled={running}>
                <SelectTrigger id="subdomain-wordlist-size" size="lg" className="w-56">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">{t('wordlist_off')}</SelectItem>
                  <SelectItem value="100">{t('wordlist_top_100')}</SelectItem>
                  <SelectItem value="1000">{t('wordlist_top_1000')}</SelectItem>
                  <SelectItem value={String(wordlistTotal)}>
                    {t('wordlist_full', { count: wordlistTotal })}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <Label htmlFor="subdomain-speed" className="text-sm text-muted-foreground">{t('speed_label')}</Label>
              <Select value={speed} onValueChange={(v) => setSpeed(v as ScanSpeed)} disabled={running}>
                <SelectTrigger id="subdomain-speed" size="lg" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(SPEED_PRESETS) as ScanSpeed[]).map((s) => (
                    <SelectItem key={s} value={s}>
                      {t(`speed_${s}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            {speed === 'fast' && t('speed_hint_fast')}
            {speed === 'normal' && t('speed_hint_normal')}
            {speed === 'cautious' && t('speed_hint_cautious')}
          </p>

          <div className="flex items-center gap-2 flex-wrap">
            {status && (
              <Badge variant="outline">
                {status === 'running' && <Loader2 className="size-3 animate-spin" />}
                {status}
                {total !== null && ` · ${results.length}${total ? `/${total}` : ''} ${t('checked_suffix')}`}
              </Badge>
            )}
            {results.length > 0 && (
              <Badge className="bg-success/15 text-success border-success/30" variant="outline">
                {foundCount} {t('found_suffix')}
              </Badge>
            )}
            {results.length > 0 && (
              <div className="flex items-center gap-2 ml-auto">
                <Switch id="subdomain-show-unresolved" checked={showUnresolved} onCheckedChange={setShowUnresolved} />
                <Label htmlFor="subdomain-show-unresolved" className="text-xs text-muted-foreground">{t('show_unresolved')}</Label>
              </div>
            )}
          </div>

          {startError && <div className="text-sm text-destructive">{startError}</div>}
          {warning && <div className="text-sm text-warning">{warning}</div>}
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-muted-foreground">{t('results_title')}</h3>
          {sorted.length === 0 ? (
            <EmptyState
              icon={Globe}
              title={t('empty_title')}
              description={t('empty_description')}
            />
          ) : (
            <div className="border rounded-lg overflow-auto max-h-[32rem]">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('subdomain')}>
                      {t('column_subdomain')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>{t('column_resolved')}</TableHead>
                    <TableHead>{t('column_ips')}</TableHead>
                    <TableHead>{t('column_source')}</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('duration_ms')}>
                      {t('column_time')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sorted.map((r) => (
                    <TableRow key={r.subdomain}>
                      <TableCell className="font-mono text-xs">{r.subdomain}</TableCell>
                      <TableCell>
                        {r.resolved ? (
                          <Badge className="bg-success/15 text-success border-success/30" variant="outline">
                            {t('badge_yes')}
                          </Badge>
                        ) : (
                          <Badge variant="secondary">{t('badge_no')}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs max-w-48 truncate">
                        {r.ips?.join(', ') ?? ''}
                      </TableCell>
                      <TableCell className="flex gap-1 flex-wrap">
                        {r.sources.map((s) => (
                          <Badge key={s} variant="secondary" className="text-[10px]">
                            {SOURCE_LABEL[s] ?? s}
                          </Badge>
                        ))}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{r.duration_ms}ms</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <SubdomainFinderGuide />
      </aside>
    </div>
  )
}
