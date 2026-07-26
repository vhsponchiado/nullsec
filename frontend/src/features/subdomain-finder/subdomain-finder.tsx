import { useEffect, useMemo, useRef, useState } from 'react'
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
import type {
  SubdomainResult,
  SubdomainResultEvent,
  SubdomainStatus,
  SubdomainTotalEvent,
} from '@/lib/types'
import { SubdomainFinderGuide } from './subdomain-finder-guide'

const SOURCE_LABEL: Record<string, string> = {
  root: 'root',
  crtsh: 'crt.sh',
  bruteforce: 'brute-force',
}

type SortKey = 'subdomain' | 'duration_ms'

type ScanSpeed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<ScanSpeed, { label: string; concurrency: number; timeoutMs: number }> = {
  fast: { label: 'Fast', concurrency: 100, timeoutMs: 2000 },
  normal: { label: 'Normal', concurrency: 50, timeoutMs: 3000 },
  cautious: { label: 'Cautious', concurrency: 10, timeoutMs: 5000 },
}

export function SubdomainFinder() {
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
      setStartError('Domain is required.')
      return
    }
    if (!useCRTSH && wordlistSize === '0') {
      setStartError('Enable crt.sh, DNS brute-force, or both.')
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
      // the scan may have already finished server-side — nothing to do
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

  const sorted = useMemo(() => {
    const copy = [...visible]
    copy.sort((a, b) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (av < bv) return sortDir === 'asc' ? -1 : 1
      if (av > bv) return sortDir === 'asc' ? 1 : -1
      return 0
    })
    return copy
  }, [visible, sortKey, sortDir])

  const foundCount = results.filter((r) => r.resolved).length
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">Target</h3>

          <div className="flex gap-2 items-end flex-wrap">
            <div className="flex flex-col gap-1.5 flex-1 min-w-48">
              <Label htmlFor="subdomain-domain">Domain</Label>
              <Input
                id="subdomain-domain"
                size="lg"
                className="font-mono"
                value={domain}
                disabled={running}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="example.com"
              />
            </div>
            {!running ? (
              <Button size="lg" onClick={handleStart} disabled={starting} className="shrink-0">
                {starting ? <Loader2 className="animate-spin" /> : <Play />}
                Start scan
              </Button>
            ) : (
              <Button size="lg" variant="destructive" onClick={handleStop} className="shrink-0">
                <Square />
                Stop
              </Button>
            )}
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <Switch id="subdomain-use-crtsh" checked={useCRTSH} onCheckedChange={setUseCRTSH} disabled={running} />
              <Label htmlFor="subdomain-use-crtsh" className="text-sm">crt.sh (passive)</Label>
            </div>
            <div className="flex items-center gap-2">
              <Label htmlFor="subdomain-wordlist-size" className="text-sm text-muted-foreground">DNS brute-force</Label>
              <Select value={wordlistSize} onValueChange={setWordlistSize} disabled={running}>
                <SelectTrigger id="subdomain-wordlist-size" size="lg" className="w-56">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">Off</SelectItem>
                  <SelectItem value="100">Top 100 prefixes</SelectItem>
                  <SelectItem value="1000">Top 1000 prefixes</SelectItem>
                  <SelectItem value={String(wordlistTotal)}>
                    Top {wordlistTotal} prefixes (full)
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <Label htmlFor="subdomain-speed" className="text-sm text-muted-foreground">Speed</Label>
              <Select value={speed} onValueChange={(v) => setSpeed(v as ScanSpeed)} disabled={running}>
                <SelectTrigger id="subdomain-speed" size="lg" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(SPEED_PRESETS) as ScanSpeed[]).map((s) => (
                    <SelectItem key={s} value={s}>
                      {SPEED_PRESETS[s].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            {speed === 'fast' && 'High concurrency, short DNS timeout — quickest, may miss slow-to-resolve names.'}
            {speed === 'normal' && 'Balanced default — good for most domains.'}
            {speed === 'cautious' && 'Low concurrency, longer DNS timeout — gentler on the nameservers.'}
          </p>

          <div className="flex items-center gap-2 flex-wrap">
            {status && (
              <Badge variant="outline">
                {status === 'running' && <Loader2 className="size-3 animate-spin" />}
                {status}
                {total !== null && ` · ${results.length}${total ? `/${total}` : ''} checked`}
              </Badge>
            )}
            {results.length > 0 && (
              <Badge className="bg-success/15 text-success border-success/30" variant="outline">
                {foundCount} found
              </Badge>
            )}
            {results.length > 0 && (
              <div className="flex items-center gap-2 ml-auto">
                <Switch id="subdomain-show-unresolved" checked={showUnresolved} onCheckedChange={setShowUnresolved} />
                <Label htmlFor="subdomain-show-unresolved" className="text-xs text-muted-foreground">Show unresolved attempts</Label>
              </div>
            )}
          </div>

          {startError && <div className="text-sm text-destructive">{startError}</div>}
          {warning && <div className="text-sm text-warning">{warning}</div>}
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-muted-foreground">Results</h3>
          {sorted.length === 0 ? (
            <EmptyState
              icon={Globe}
              title="No results yet"
              description="Enter a domain above and click Start scan."
            />
          ) : (
            <div className="border rounded-lg overflow-auto max-h-[32rem]">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('subdomain')}>
                      Subdomain <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>Resolved</TableHead>
                    <TableHead>IP(s)</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('duration_ms')}>
                      Time <ArrowUpDown className="inline size-3" />
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
                            yes
                          </Badge>
                        ) : (
                          <Badge variant="secondary">no</Badge>
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
