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
import { Play, Square, Loader2, ArrowUpDown, FolderSearch } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import { useSortedResults } from '@/lib/use-sorted-results'
import type { DirFinderResult, DirFinderResultEvent, DirFinderStatus } from '@/lib/types'
import { DirFinderGuide } from './dir-finder-guide'

type SortKey = 'path' | 'status' | 'size' | 'duration_ms'

type ScanSpeed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<ScanSpeed, { concurrency: number; timeoutMs: number }> = {
  fast: { concurrency: 50, timeoutMs: 3000 },
  normal: { concurrency: 20, timeoutMs: 5000 },
  cautious: { concurrency: 5, timeoutMs: 10000 },
}

function statusBadgeClass(status: number): string {
  if (status >= 200 && status < 300) return 'bg-success/15 text-success border-success/30'
  if (status >= 300 && status < 400) return 'bg-warning/15 text-warning border-warning/30'
  if (status === 401 || status === 403) return 'bg-warning/15 text-warning border-warning/30'
  return ''
}

export function DirFinder() {
  const { t } = useTranslation('dir-finder')
  const [target, setTarget] = useState('')
  const [wordlistSize, setWordlistSize] = useState('1000')
  const [wordlistTotal, setWordlistTotal] = useState(4751)
  const [extensions, setExtensions] = useState('')
  const [speed, setSpeed] = useState<ScanSpeed>('normal')
  const [show404s, setShow404s] = useState(false)

  useEffect(() => {
    api.dirfinderWordlistSize().then((r) => setWordlistTotal(r.size)).catch(() => {})
  }, [])

  const [scanId, setScanId] = useState<string | null>(null)
  const [status, setStatus] = useState<DirFinderStatus | null>(null)
  const [results, setResults] = useState<DirFinderResult[]>([])
  const [total, setTotal] = useState<number | null>(null)
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('path')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const scanIdRef = useRef<string | null>(null)

  function mergeResults(incoming: DirFinderResult[] | null | undefined) {
    setResults((prev) => {
      const known = new Set(prev.map((r) => r.path))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.path))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'dirfinder_result') {
      const payload = event.payload as DirFinderResultEvent
      if (payload.scan_id !== scanIdRef.current) return
      mergeResults([payload.result])
    } else if (event.type === 'dirfinder_done' || event.type === 'dirfinder_stopped') {
      const payload = event.payload as { scan_id: string }
      if (payload.scan_id !== scanIdRef.current) return
      setStatus(event.type === 'dirfinder_done' ? 'done' : 'stopped')
    }
  })

  async function handleStart() {
    if (!target.trim()) {
      setStartError(t('target_required'))
      return
    }

    setStarting(true)
    setStartError(null)
    setResults([])
    setStatus(null)
    setTotal(null)
    try {
      const preset = SPEED_PRESETS[speed]
      const exts = extensions
        .split(',')
        .map((e) => e.trim())
        .filter(Boolean)
      const resp = await api.startDirFinderScan({
        target: target.trim(),
        wordlist_size: parseInt(wordlistSize, 10),
        extensions: exts.length ? exts : undefined,
        concurrency: preset.concurrency,
        timeout_ms: preset.timeoutMs,
      })
      scanIdRef.current = resp.scan_id
      setScanId(resp.scan_id)
      setStatus('running')
      setTotal(resp.total)
      try {
        const snapshot = await api.getDirFinderScan(resp.scan_id)
        mergeResults(snapshot.results)
        setStatus(snapshot.status)
        setTotal(snapshot.total || resp.total)
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
      await api.stopDirFinderScan(scanId)
    } catch {
      // the scan may have already finished server-side, nothing to do
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

  const visible = show404s ? results : results.filter((r) => r.status !== 404 && r.status !== 0)

  const compare = useCallback(
    (a: DirFinderResult, b: DirFinderResult) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (av < bv) return sortDir === 'asc' ? -1 : 1
      if (av > bv) return sortDir === 'asc' ? 1 : -1
      return 0
    },
    [sortKey, sortDir],
  )
  const sorted = useSortedResults(visible, compare)

  const foundCount = results.filter((r) => r.status !== 404 && r.status !== 0).length
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">{t('target_section_title')}</h3>

          <div className="flex gap-2 items-end flex-wrap">
            <div className="flex flex-col gap-1.5 flex-1 min-w-48">
              <Label htmlFor="dirfinder-target">{t('target_url_label')}</Label>
              <Input
                id="dirfinder-target"
                size="lg"
                className="font-mono"
                value={target}
                disabled={running}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={t('target_placeholder')}
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
              <Label htmlFor="dirfinder-wordlist-size" className="text-sm text-muted-foreground">{t('wordlist_label')}</Label>
              <Select value={wordlistSize} onValueChange={setWordlistSize} disabled={running}>
                <SelectTrigger id="dirfinder-wordlist-size" size="lg" className="w-56">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="100">{t('wordlist_top_100')}</SelectItem>
                  <SelectItem value="1000">{t('wordlist_top_1000')}</SelectItem>
                  <SelectItem value={String(wordlistTotal)}>
                    {t('wordlist_full', { count: wordlistTotal })}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <Label htmlFor="dirfinder-speed" className="text-sm text-muted-foreground">{t('speed_label')}</Label>
              <Select value={speed} onValueChange={(v) => setSpeed(v as ScanSpeed)} disabled={running}>
                <SelectTrigger id="dirfinder-speed" size="lg" className="w-32">
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
            <div className="flex flex-col gap-1.5 min-w-48">
              <Label htmlFor="dirfinder-extensions" className="text-xs text-muted-foreground">
                {t('extensions_label')}
              </Label>
              <Input
                id="dirfinder-extensions"
                className="font-mono"
                value={extensions}
                disabled={running}
                onChange={(e) => setExtensions(e.target.value)}
                placeholder={t('extensions_placeholder')}
              />
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
                <Switch id="dirfinder-show-404s" checked={show404s} onCheckedChange={setShow404s} />
                <Label htmlFor="dirfinder-show-404s" className="text-xs text-muted-foreground">{t('show_404s')}</Label>
              </div>
            )}
          </div>

          {startError && <div className="text-sm text-destructive">{startError}</div>}
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-muted-foreground">{t('results_title')}</h3>
          {sorted.length === 0 ? (
            <EmptyState
              icon={FolderSearch}
              title={t('empty_title')}
              description={t('empty_description')}
            />
          ) : (
            <div className="border rounded-lg overflow-auto max-h-[32rem]">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('path')}>
                      {t('column_path')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('status')}>
                      {t('column_status')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('size')}>
                      {t('column_size')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>{t('column_redirect')}</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('duration_ms')}>
                      {t('column_time')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sorted.map((r) => (
                    <TableRow key={r.path}>
                      <TableCell className="font-mono text-xs">/{r.path}</TableCell>
                      <TableCell>
                        {r.error ? (
                          <Badge variant="destructive">{t('error_badge')}</Badge>
                        ) : (
                          <Badge className={statusBadgeClass(r.status)} variant="outline">
                            {r.status}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{r.size}</TableCell>
                      <TableCell className="font-mono text-xs max-w-48 truncate">
                        {r.redirect ?? ''}
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
        <DirFinderGuide />
      </aside>
    </div>
  )
}
