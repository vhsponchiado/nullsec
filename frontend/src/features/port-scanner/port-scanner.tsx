import { useEffect, useMemo, useRef, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
import { Play, Square, Loader2, ArrowUpDown, Radar } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import type {
  PortScanPresets,
  PortScanResult,
  PortScanResultEvent,
  PortScanState,
  PortScanStatus,
} from '@/lib/types'
import { PortScannerGuide } from './port-scanner-guide'

type PortMode = 'top100' | 'top1000' | 'custom'

function parsePortSpec(spec: string): { ports: number[]; error: string | null } {
  const seen = new Set<number>()
  const out: number[] = []
  const parts = spec.split(',').map((s) => s.trim()).filter(Boolean)
  for (const part of parts) {
    const m = /^(\d+)(?:-(\d+))?$/.exec(part)
    if (!m) return { ports: [], error: `Invalid port spec "${part}"` }
    const start = parseInt(m[1], 10)
    const end = m[2] ? parseInt(m[2], 10) : start
    if (start < 1 || end > 65535 || end < start) {
      return { ports: [], error: `Port range "${part}" out of bounds (1-65535)` }
    }
    for (let p = start; p <= end; p++) {
      if (!seen.has(p)) {
        seen.add(p)
        out.push(p)
      }
    }
  }
  if (out.length === 0) return { ports: [], error: 'No ports specified' }
  out.sort((a, b) => a - b)
  return { ports: out, error: null }
}

const STATE_VARIANT: Record<PortScanState, 'default' | 'secondary' | 'outline'> = {
  open: 'default',
  closed: 'secondary',
  filtered: 'outline',
}

const STATE_CLASS: Record<PortScanState, string> = {
  open: 'bg-success/15 text-success border-success/30',
  closed: '',
  filtered: 'border-warning text-warning',
}

type ScanSpeed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<ScanSpeed, { label: string; concurrency: number; timeoutMs: number }> = {
  fast: { label: 'Fast', concurrency: 400, timeoutMs: 500 },
  normal: { label: 'Normal', concurrency: 200, timeoutMs: 800 },
  cautious: { label: 'Cautious', concurrency: 20, timeoutMs: 2000 },
}

type SortKey = 'port' | 'state' | 'duration_ms'

export function PortScanner() {
  const [host, setHost] = useState('')
  const [mode, setMode] = useState<PortMode>('top100')
  const [customSpec, setCustomSpec] = useState('1-1024')
  const [speed, setSpeed] = useState<ScanSpeed>('normal')
  const [presets, setPresets] = useState<PortScanPresets | null>(null)
  const top100Count = presets?.top100.length ?? 100
  const top1000Count = presets?.top1000.length ?? 1000

  useEffect(() => {
    api.portscanPresets().then(setPresets).catch(() => {})
  }, [])

  const { ports: customPorts, error: customError } = useMemo(
    () => (mode === 'custom' ? parsePortSpec(customSpec) : { ports: [], error: null }),
    [mode, customSpec],
  )

  const portCount = mode === 'top100' ? top100Count : mode === 'top1000' ? top1000Count : customPorts.length

  const [scanId, setScanId] = useState<string | null>(null)
  const [status, setStatus] = useState<PortScanStatus | null>(null)
  const [results, setResults] = useState<PortScanResult[]>([])
  const [total, setTotal] = useState<number | null>(null)
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('port')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const scanIdRef = useRef<string | null>(null)

  function mergeResults(incoming: PortScanResult[] | null | undefined) {
    setResults((prev) => {
      const known = new Set(prev.map((r) => r.port))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.port))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'portscan_result') {
      const payload = event.payload as PortScanResultEvent
      if (payload.scan_id !== scanIdRef.current) return
      mergeResults([payload.result])
    } else if (event.type === 'portscan_done' || event.type === 'portscan_stopped') {
      const payload = event.payload as { scan_id: string }
      if (payload.scan_id !== scanIdRef.current) return
      setStatus(event.type === 'portscan_done' ? 'done' : 'stopped')
    }
  })

  async function handleStart() {
    if (!host.trim()) {
      setStartError('Host is required.')
      return
    }
    const ports = mode === 'top100' ? null : mode === 'top1000' ? null : customPorts
    if (mode === 'custom' && (customError || customPorts.length === 0)) {
      setStartError(customError ?? 'No ports specified.')
      return
    }

    setStarting(true)
    setStartError(null)
    setResults([])
    setStatus(null)
    setTotal(null)
    try {
      let portList: number[]
      if (mode === 'custom') {
        portList = ports ?? customPorts
      } else if (presets) {
        portList = (mode === 'top100' ? presets.top100 : presets.top1000).map((p) => p.port)
      } else {
        const fetched = await api.portscanPresets()
        setPresets(fetched)
        portList = (mode === 'top100' ? fetched.top100 : fetched.top1000).map((p) => p.port)
      }

      const preset = SPEED_PRESETS[speed]
      const resp = await api.startPortScan({
        host: host.trim(),
        ports: portList,
        concurrency: preset.concurrency,
        timeout_ms: preset.timeoutMs,
      })
      scanIdRef.current = resp.scan_id
      setScanId(resp.scan_id)
      setStatus('running')
      setTotal(resp.total)
      try {
        const snapshot = await api.getPortScan(resp.scan_id)
        mergeResults(snapshot.results)
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
    if (!scanId) return
    try {
      await api.stopPortScan(scanId)
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

  const sorted = useMemo(() => {
    const copy = [...results]
    copy.sort((a, b) => {
      let av: number | string = a[sortKey]
      let bv: number | string = b[sortKey]
      if (sortKey === 'state') {
        av = a.state
        bv = b.state
      }
      if (av < bv) return sortDir === 'asc' ? -1 : 1
      if (av > bv) return sortDir === 'asc' ? 1 : -1
      return 0
    })
    return copy
  }, [results, sortKey, sortDir])

  const openCount = results.filter((r) => r.state === 'open').length
  const closedCount = results.filter((r) => r.state === 'closed').length
  const filteredCount = results.filter((r) => r.state === 'filtered').length
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">Target</h3>

          <div className="flex gap-2 items-end flex-wrap">
            <div className="flex flex-col gap-1.5 flex-1 min-w-48">
              <Label htmlFor="portscan-host">Host</Label>
              <Input
                id="portscan-host"
                size="lg"
                className="font-mono"
                value={host}
                disabled={running}
                onChange={(e) => setHost(e.target.value)}
                placeholder="scanme.nmap.org"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="portscan-mode">Ports</Label>
              <Select value={mode} onValueChange={(v) => setMode(v as PortMode)} disabled={running}>
                <SelectTrigger id="portscan-mode" size="lg" className="w-56">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="top100">Top {top100Count} ports</SelectItem>
                  <SelectItem value="top1000">Top {top1000Count} ports</SelectItem>
                  <SelectItem value="custom">Custom range</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="portscan-speed">Speed</Label>
              <Select value={speed} onValueChange={(v) => setSpeed(v as ScanSpeed)} disabled={running}>
                <SelectTrigger id="portscan-speed" size="lg" className="w-36">
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

          {mode === 'custom' && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="portscan-custom-spec">Custom port spec</Label>
              <Input
                id="portscan-custom-spec"
                size="lg"
                className="font-mono"
                value={customSpec}
                disabled={running}
                onChange={(e) => setCustomSpec(e.target.value)}
                placeholder="22,80,443,8000-8100"
              />
              {customError && <p className="text-xs text-destructive">{customError}</p>}
            </div>
          )}

          <p className="text-xs text-muted-foreground">
            {speed === 'fast' && 'High concurrency, short timeout — quickest, but may miss slow/filtered ports.'}
            {speed === 'normal' && 'Balanced default — good for most targets.'}
            {speed === 'cautious' && 'Low concurrency, longer timeout — gentler on the target and its rate limits.'}
          </p>

          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant="secondary">{portCount.toLocaleString()} ports</Badge>
            {status && (
              <Badge variant="outline">
                {status === 'running' && <Loader2 className="size-3 animate-spin" />}
                {status}
                {total !== null && ` · ${results.length}/${total} done`}
              </Badge>
            )}
            {results.length > 0 && (
              <>
                <Badge className="bg-success/15 text-success border-success/30" variant="outline">
                  {openCount} open
                </Badge>
                <Badge variant="secondary">{closedCount} closed</Badge>
                <Badge variant="outline" className="border-warning text-warning">
                  {filteredCount} filtered
                </Badge>
              </>
            )}
          </div>

          {startError && <div className="text-sm text-destructive">{startError}</div>}
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-muted-foreground">Results</h3>
          {results.length === 0 ? (
            <EmptyState
              icon={Radar}
              title="No results yet"
              description="Set a target above and click Start scan."
            />
          ) : (
            <div className="border rounded-lg overflow-auto max-h-[32rem]">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('port')}>
                      Port <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('state')}>
                      State <ArrowUpDown className="inline size-3" />
                    </TableHead>
                    <TableHead>Service</TableHead>
                    <TableHead>Banner</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('duration_ms')}>
                      Time <ArrowUpDown className="inline size-3" />
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sorted.map((r) => (
                    <TableRow key={r.port}>
                      <TableCell className="font-mono text-xs">{r.port}</TableCell>
                      <TableCell>
                        <Badge variant={STATE_VARIANT[r.state]} className={STATE_CLASS[r.state]}>
                          {r.state}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{r.service}</TableCell>
                      <TableCell className="font-mono text-xs max-w-64 truncate">
                        {r.banner ?? ''}
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
        <PortScannerGuide />
      </aside>
    </div>
  )
}
