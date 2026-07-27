import { useCallback, useRef, useState } from 'react'
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
import { Play, Square, Loader2, ArrowUpDown, Waypoints } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import { useSortedResults } from '@/lib/use-sorted-results'
import type { SpiderPage, SpiderPageEvent, SpiderStatus } from '@/lib/types'
import { SpiderGuide } from './spider-guide'

type SortKey = 'url' | 'status' | 'depth' | 'duration_ms'
type Speed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<Speed, { concurrency: number }> = {
  fast: { concurrency: 15 },
  normal: { concurrency: 5 },
  cautious: { concurrency: 2 },
}

export function Spider() {
  const { t } = useTranslation('spider')
  const [url, setUrl] = useState('')
  const [maxDepth, setMaxDepth] = useState('3')
  const [maxPages, setMaxPages] = useState('200')
  const [includeSubdomains, setIncludeSubdomains] = useState(false)
  const [speed, setSpeed] = useState<Speed>('normal')

  const [spiderId, setSpiderId] = useState<string | null>(null)
  const [status, setStatus] = useState<SpiderStatus | null>(null)
  const [pages, setPages] = useState<SpiderPage[]>([])
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('depth')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const spiderIdRef = useRef<string | null>(null)

  function mergePages(incoming: SpiderPage[] | null | undefined) {
    setPages((prev) => {
      const known = new Set(prev.map((p) => p.url))
      const fresh = (incoming ?? []).filter((p) => !known.has(p.url))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'spider_page') {
      const payload = event.payload as SpiderPageEvent
      if (payload.spider_id !== spiderIdRef.current) return
      mergePages([payload.page])
    } else if (event.type === 'spider_done' || event.type === 'spider_stopped') {
      const payload = event.payload as { spider_id: string }
      if (payload.spider_id !== spiderIdRef.current) return
      setStatus(event.type === 'spider_done' ? 'done' : 'stopped')
    }
  })

  async function handleStart() {
    if (!url.trim()) {
      setStartError(t('url_required'))
      return
    }
    setStarting(true)
    setStartError(null)
    setPages([])
    setStatus(null)
    try {
      const resp = await api.startSpider({
        url: url.trim(),
        max_depth: parseInt(maxDepth, 10) || undefined,
        max_pages: parseInt(maxPages, 10) || undefined,
        include_subdomains: includeSubdomains,
        concurrency: SPEED_PRESETS[speed].concurrency,
      })
      spiderIdRef.current = resp.spider_id
      setSpiderId(resp.spider_id)
      setStatus('running')
      try {
        const snapshot = await api.getSpider(resp.spider_id)
        mergePages(snapshot.pages)
        setStatus(snapshot.status)
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
    if (!spiderId) return
    try {
      await api.stopSpider(spiderId)
    } catch {
      // the crawl may have already finished server-side, so there's nothing to do
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

  const compare = useCallback(
    (a: SpiderPage, b: SpiderPage) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (av < bv) return sortDir === 'asc' ? -1 : 1
      if (av > bv) return sortDir === 'asc' ? 1 : -1
      return 0
    },
    [sortKey, sortDir],
  )
  const sorted = useSortedResults(pages, compare)

  const formsFound = pages.reduce((acc, p) => acc + (p.forms?.length ?? 0), 0)
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">{t('target_section_title')}</h3>

          <div className="flex gap-2 items-end flex-wrap">
            <div className="flex flex-col gap-1.5 flex-1 min-w-48">
              <Label htmlFor="spider-url">{t('start_url_label')}</Label>
              <Input
                id="spider-url"
                size="lg"
                className="font-mono"
                value={url}
                disabled={running}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t('start_url_placeholder')}
              />
            </div>
            {!running ? (
              <Button size="lg" onClick={handleStart} disabled={starting} className="shrink-0">
                {starting ? <Loader2 className="animate-spin" /> : <Play />}
                {t('start_crawl')}
              </Button>
            ) : (
              <Button size="lg" variant="destructive" onClick={handleStop} className="shrink-0">
                <Square />
                {t('stop')}
              </Button>
            )}
          </div>

          <div className="flex items-end gap-3 flex-wrap">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="spider-max-depth" className="text-xs">{t('max_depth_label')}</Label>
              <Input
                id="spider-max-depth"
                size="lg"
                className="w-24 font-mono"
                value={maxDepth}
                disabled={running}
                onChange={(e) => setMaxDepth(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="spider-max-pages" className="text-xs">{t('max_pages_label')}</Label>
              <Input
                id="spider-max-pages"
                size="lg"
                className="w-28 font-mono"
                value={maxPages}
                disabled={running}
                onChange={(e) => setMaxPages(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="spider-speed" className="text-xs">{t('speed_label')}</Label>
              <Select value={speed} onValueChange={(v) => setSpeed(v as Speed)} disabled={running}>
                <SelectTrigger id="spider-speed" size="lg" className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(SPEED_PRESETS) as Speed[]).map((s) => (
                    <SelectItem key={s} value={s}>
                      {t(`speed_${s}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-2 pb-2.5">
              <Switch id="spider-include-subdomains" checked={includeSubdomains} onCheckedChange={setIncludeSubdomains} disabled={running} />
              <Label htmlFor="spider-include-subdomains" className="text-sm">{t('include_subdomains')}</Label>
            </div>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {status && (
              <Badge variant="outline">
                {status === 'running' && <Loader2 className="size-3 animate-spin" />}
                {status}
                {pages.length > 0 && ` · ${t('pages_count', { count: pages.length })}`}
              </Badge>
            )}
            {formsFound > 0 && <Badge variant="secondary">{t('forms_found', { count: formsFound })}</Badge>}
          </div>

          {startError && <div className="text-sm text-destructive">{startError}</div>}
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-muted-foreground">
            {t('results_title')}
          </h3>
          {sorted.length === 0 ? (
            <EmptyState
              icon={Waypoints}
              title={t('empty_title')}
              description={t('empty_description')}
            />
          ) : (
            <div className="border rounded-lg overflow-auto max-h-[32rem]">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('url')}>
                      {t('column_url')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('status')}>
                      {t('column_status')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>{t('column_title')}</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('depth')}>
                      {t('column_depth')} <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>{t('column_links')}</TableHead>
                    <TableHead>{t('column_forms')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sorted.map((p) => (
                    <TableRow key={p.url}>
                      <TableCell className="font-mono text-xs max-w-72 truncate">{p.url}</TableCell>
                      <TableCell>
                        {p.error ? (
                          <Badge variant="outline">{t('error_badge')}</Badge>
                        ) : (
                          <Badge variant={p.status >= 400 ? 'outline' : 'secondary'}>{p.status}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-xs max-w-40 truncate">{p.title ?? ''}</TableCell>
                      <TableCell className="font-mono text-xs">{p.depth}</TableCell>
                      <TableCell className="font-mono text-xs">{p.links_found}</TableCell>
                      <TableCell className="font-mono text-xs">{p.forms?.length ?? 0}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <SpiderGuide />
      </aside>
    </div>
  )
}
