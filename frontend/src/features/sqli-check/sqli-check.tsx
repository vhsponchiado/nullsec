import { useEffect, useRef, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ResponseView } from '@/components/response-view'
import { Switch } from '@/components/ui/switch'
import {
  HeadersEditor,
  headerRowsToHeaders,
  headersToHeaderRows,
  type HeaderRow,
} from '@/components/headers-editor'
import { EmptyState } from '@/components/empty-state'
import {
  ShieldAlert,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Copy,
  Check,
  Play,
  Square,
  ChevronDown,
  ChevronUp,
  Bug,
  SendHorizontal,
} from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import { textToHeaders, parseJSONHeaders } from '@/lib/headers'
import { groupBy } from '@/lib/utils'
import { useClipboardCopy } from '@/lib/use-clipboard-copy'
import type {
  RepeaterResponse,
  SqliPayload,
  SqliReport,
  SqliScanItemResult,
  SqliScanResultEvent,
  SqliScanStatus,
  SqliSeverity,
} from '@/lib/types'
import type { RequestSeed } from '@/components/request-editor'
import { SqliGuide } from './sqli-check-guide'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

const TYPE_LABEL: Record<string, string> = {
  error: 'Error-based',
  boolean: 'Boolean-based',
  time: 'Time-based',
}

const SEVERITY_ICON: Record<SqliSeverity, React.ReactNode> = {
  critical: <ShieldAlert className="size-4 text-destructive shrink-0" />,
  warning: <AlertTriangle className="size-4 text-warning shrink-0" />,
  info: <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />,
}

function detectParams(url: string, body: string): Array<{ location: 'query' | 'body'; name: string }> {
  const out: Array<{ location: 'query' | 'body'; name: string }> = []
  try {
    const u = new URL(url)
    for (const name of u.searchParams.keys()) out.push({ location: 'query', name })
  } catch {
    // invalid/incomplete URL — no query params yet
  }
  if (body.trim()) {
    try {
      const params = new URLSearchParams(body)
      for (const name of params.keys()) out.push({ location: 'body', name })
    } catch {
      // not form-urlencoded — ignore
    }
  }
  return out
}

function SqliReportView({ report }: { report: SqliReport }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <div className="flex gap-2 items-center flex-wrap">
          <Badge variant="secondary">{report.summary.params_tested} parameters</Badge>
          <Badge variant="secondary">{report.summary.requests} requests</Badge>
          <Badge variant="secondary">{report.summary.vulnerable} signals found</Badge>
          {report.summary.critical > 0 && (
            <Badge variant="outline" className="border-destructive text-destructive">
              {report.summary.critical} critical
            </Badge>
          )}
        </div>
        <span className="text-xs text-muted-foreground">
          baseline: status {report.baseline.status || '—'}, {report.baseline.duration_ms}
          ms, {report.baseline.content_length} bytes
        </span>
      </div>

      {report.summary.params_tested === 0 && (
        <div className="text-sm text-muted-foreground border rounded-lg p-4">
          No parameters found in the URL (query string) or the body. Add some to test, e.g.{' '}
          <code>?id=1</code>.
        </div>
      )}

      {report.boolean_pairs && report.boolean_pairs.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h3 className="text-sm font-medium">True/false test</h3>
            <p className="text-xs text-muted-foreground">
              We send a condition that should always be <strong>true</strong> and another that's
              always <strong>false</strong>. If the site responds differently to each one, that's
              a strong signal of SQL injection. If it responds the same to both, there's no signal
              here.
            </p>
          </div>
          {groupBy(report.boolean_pairs, (p) => `${p.location}:${p.parameter}`).map(([key, pairs]) => {
            const [location, parameter] = key.split(':')
            const vulnerableCount = pairs.filter((p) => p.vulnerable).length
            const isVulnerable = vulnerableCount > 0
            return (
              <div key={key} className="border rounded-lg min-w-0">
                <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
                  <span className="font-mono">
                    {parameter} <span className="text-muted-foreground">({location})</span>
                  </span>
                  {isVulnerable ? (
                    <Badge variant="outline" className="border-destructive text-destructive">
                      Possible SQL injection
                    </Badge>
                  ) : (
                    <Badge variant="secondary">No signal</Badge>
                  )}
                </div>
                <div className="divide-y">
                  {pairs.map((p, i) => (
                    <div key={i} className="flex items-start gap-3 px-4 py-3 min-w-0">
                      <div className="pt-0.5">
                        {p.error ? (
                          <AlertTriangle className="size-4 text-muted-foreground shrink-0" />
                        ) : (
                          SEVERITY_ICON[p.severity]
                        )}
                      </div>
                      <div className="min-w-0 flex-1 flex flex-col gap-1">
                        <p className="text-xs break-words">{p.error || p.note}</p>
                        <p className="text-[11px] text-muted-foreground font-mono break-all">
                          true → status {p.true_status || '—'} ({p.true_content_length} bytes) ·
                          false → status {p.false_status || '—'} ({p.false_content_length} bytes)
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}

      <Separator />

      <div className="flex flex-col gap-3">
        <h3 className="text-sm font-medium">Database error and response delay</h3>
        <p className="text-xs text-muted-foreground">
          Each row below only shows up if some payload caused a visible database error in the
          response, or took much longer than normal to respond (time-based "blind" SQLi signal).
        </p>
      </div>

      {groupBy(report.findings, (f) => `${f.location}:${f.parameter}`).map(([key, findings]) => {
        const [location, parameter] = key.split(':')
        const vulnerableCount = findings.filter((f) => f.vulnerable).length
        return (
          <div key={key} className="border rounded-lg min-w-0">
            <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
              <span className="font-mono">
                {parameter} <span className="text-muted-foreground">({location})</span>
              </span>
              {vulnerableCount > 0 && (
                <Badge variant="outline" className="border-destructive text-destructive">
                  {vulnerableCount} signals
                </Badge>
              )}
            </div>
            <div className="divide-y">
              {findings
                .filter((f) => f.vulnerable || f.error)
                .map((f, i) => (
                  <div key={i} className="flex items-start gap-3 px-4 py-3 min-w-0">
                    <div className="pt-0.5">
                      {f.error ? (
                        <AlertTriangle className="size-4 text-muted-foreground shrink-0" />
                      ) : (
                        SEVERITY_ICON[f.severity]
                      )}
                    </div>
                    <div className="min-w-0 flex-1 flex flex-col gap-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-mono text-xs font-medium break-all">{f.payload}</span>
                        <Badge variant="secondary">{f.payload_type}</Badge>
                        <span className="font-mono text-xs text-muted-foreground">
                          {f.status || '—'} · {f.duration_ms}ms · {f.content_length}b
                        </span>
                      </div>
                      <p className="text-xs text-muted-foreground break-words">{f.error || f.note}</p>
                    </div>
                  </div>
                ))}
              {findings.every((f) => !f.vulnerable && !f.error) && (
                <div className="px-4 py-3 text-xs text-muted-foreground">
                  No signal across {findings.length} payloads tested.
                </div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function SqliScanItemView({
  item,
  onTest,
}: {
  item: SqliScanItemResult
  onTest: (item: SqliScanItemResult) => void
}) {
  const [open, setOpen] = useState(false)
  const vulnerable = item.report.summary.vulnerable
  return (
    <div className="border rounded-lg min-w-0">
      <div className="w-full flex items-center justify-between gap-3 px-4 py-2.5">
        <button
          type="button"
          className="flex-1 min-w-0 text-left"
          onClick={() => setOpen((v) => !v)}
        >
          <span className="font-mono text-xs truncate block min-w-0">
            <span className="text-muted-foreground">{item.method}</span> {item.url}
          </span>
        </button>
        <div className="flex items-center gap-2 shrink-0">
          {vulnerable > 0 ? (
            <Badge variant="outline" className="border-destructive text-destructive">
              {vulnerable} signal{vulnerable === 1 ? '' : 's'}
            </Badge>
          ) : (
            <Badge variant="secondary">No signal</Badge>
          )}
          <Button
            size="sm"
            variant="outline"
            onClick={() => onTest(item)}
            title="Load this request into Automatic mode above for further manual testing"
          >
            <SendHorizontal />
            Test
          </Button>
          <button type="button" onClick={() => setOpen((v) => !v)} aria-label={open ? 'Collapse' : 'Expand'}>
            {open ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
          </button>
        </div>
      </div>
      {open && (
        <div className="border-t p-4">
          <SqliReportView report={item.report} />
        </div>
      )}
    </div>
  )
}

interface SqliCheckProps {
  seed?: RequestSeed | null
}

export function SqliCheck({ seed }: SqliCheckProps) {
  const [mode, setMode] = useState<'auto' | 'manual' | 'scan'>('auto')

  const [method, setMethod] = useState('GET')
  const [url, setUrl] = useState('')
  const [body, setBody] = useState('')
  const [headerRows, setHeaderRows] = useState<HeaderRow[]>([{ key: '', value: '' }])
  const lastNonce = useRef<number | null>(null)

  useEffect(() => {
    if (seed && seed.nonce !== lastNonce.current) {
      lastNonce.current = seed.nonce
      setMethod(seed.value.method)
      setUrl(seed.value.url)
      setBody(seed.value.body)
      setHeaderRows(headersToHeaderRows(textToHeaders(seed.value.headersText)))
      setMode('auto')
    }
  }, [seed])

  // automatic mode
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<SqliReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  // manual mode
  const [payloads, setPayloads] = useState<SqliPayload[]>([])
  const { copiedValue, copy } = useClipboardCopy<string>()
  const [manualParam, setManualParam] = useState<string | null>(null)
  const [manualPayload, setManualPayload] = useState('')
  const [manualResponse, setManualResponse] = useState<RepeaterResponse | null>(null)
  const [manualSending, setManualSending] = useState(false)
  const [manualError, setManualError] = useState<string | null>(null)

  // scan mode — batch-runs the check across every matching Proxy History entry
  const [scanHost, setScanHost] = useState('')
  const [scanId, setScanId] = useState<string | null>(null)
  const [scanStatus, setScanStatus] = useState<SqliScanStatus | null>(null)
  const [scanTotal, setScanTotal] = useState<number | null>(null)
  const [scanResults, setScanResults] = useState<SqliScanItemResult[]>([])
  const [scanStarting, setScanStarting] = useState(false)
  const [scanStartError, setScanStartError] = useState<string | null>(null)
  const [scanOnlyVulnerable, setScanOnlyVulnerable] = useState(false)
  const scanIdRef = useRef<string | null>(null)

  useEffect(() => {
    api.listSqliPayloads().then(setPayloads).catch(() => setPayloads([]))
  }, [])

  const params = detectParams(url, body)

  function mergeScanResults(incoming: SqliScanItemResult[] | null | undefined) {
    setScanResults((prev) => {
      const known = new Set(prev.map((r) => r.history_id))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.history_id))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'sqliscan_result') {
      const payload = event.payload as SqliScanResultEvent
      if (payload.scan_id !== scanIdRef.current) return
      mergeScanResults([payload.result])
    } else if (event.type === 'sqliscan_done' || event.type === 'sqliscan_stopped') {
      const payload = event.payload as { scan_id: string }
      if (payload.scan_id !== scanIdRef.current) return
      setScanStatus(event.type === 'sqliscan_done' ? 'done' : 'stopped')
    }
  })

  async function handleScanStart() {
    setScanStarting(true)
    setScanStartError(null)
    setScanResults([])
    setScanStatus(null)
    setScanTotal(null)
    try {
      const resp = await api.startSqliScan({ host: scanHost.trim() || undefined })
      scanIdRef.current = resp.scan_id
      setScanId(resp.scan_id)
      setScanStatus('running')
      setScanTotal(resp.total)
      try {
        const snapshot = await api.getSqliScan(resp.scan_id)
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
      await api.stopSqliScan(scanId)
    } catch {
      // the scan may have already finished server-side — nothing to do
    }
  }

  // Loads a scan result's original request (fetched fresh since ItemResult
  // only carries method/URL/report, not headers/body) into Automatic mode
  // for focused manual follow-up on that one finding.
  async function handleOpenInChecker(item: SqliScanItemResult) {
    try {
      const full = await api.getHistoryItem(item.history_id)
      setMethod(full.method)
      setUrl(full.url)
      setBody(full.body)
      setHeaderRows(headersToHeaderRows(parseJSONHeaders(full.headers)))
      setMode('auto')
    } catch (err) {
      setScanStartError((err as Error).message)
    }
  }

  async function handleUsePayload(value: string) {
    setManualPayload(value)
    await copy(value, value)
  }

  async function handleScan() {
    if (!url.trim()) return
    setLoading(true)
    setError(null)
    try {
      const result = await api.sqliCheck({
        method,
        url: url.trim(),
        body: body || undefined,
        headers: headerRowsToHeaders(headerRows),
      })
      setReport(result)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  async function handleManualSend() {
    if (!manualParam || !manualPayload) return
    const [location, name] = manualParam.split(':')
    let finalUrl = url
    let finalBody = body
    try {
      if (location === 'query') {
        const u = new URL(url)
        u.searchParams.set(name, manualPayload)
        finalUrl = u.toString()
      } else {
        const p = new URLSearchParams(body)
        p.set(name, manualPayload)
        finalBody = p.toString()
      }
    } catch (err) {
      setManualError((err as Error).message)
      return
    }

    setManualSending(true)
    setManualError(null)
    try {
      const resp = await api.repeaterSend({
        method,
        url: finalUrl,
        headers: headerRowsToHeaders(headerRows),
        body: finalBody,
      })
      setManualResponse(resp)
    } catch (err) {
      setManualError((err as Error).message)
    } finally {
      setManualSending(false)
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
        <Tabs value={mode} onValueChange={(v) => setMode(v as 'auto' | 'manual' | 'scan')}>
          <TabsList variant="line" className="w-fit">
            <TabsTrigger value="auto">Automatic</TabsTrigger>
            <TabsTrigger value="manual">Manual</TabsTrigger>
            <TabsTrigger value="scan">Scan (History)</TabsTrigger>
          </TabsList>
        </Tabs>

        {mode === 'scan' && (
          <div className="flex flex-col gap-4 border rounded-lg p-4">
            <h3 className="text-sm font-medium text-muted-foreground">Batch scan</h3>
            <p className="text-xs text-muted-foreground">
              Runs the same check against every request already captured in Proxy History,
              instead of one URL at a time. Filter by host to keep it scoped to a target you're
              authorized to test — leave it blank to scan everything in History.
            </p>

            <div className="flex gap-2 items-end flex-wrap">
              <div className="flex flex-col gap-1.5 flex-1 min-w-48">
                <Label htmlFor="sqli-scan-host">Host filter (optional)</Label>
                <Input
                  id="sqli-scan-host"
                  size="lg"
                  className="font-mono"
                  value={scanHost}
                  disabled={scanStatus === 'running'}
                  onChange={(e) => setScanHost(e.target.value)}
                  placeholder="authorized-target.com"
                />
              </div>
              {scanStatus !== 'running' ? (
                <Button size="lg" onClick={handleScanStart} disabled={scanStarting} className="shrink-0">
                  {scanStarting ? <Loader2 className="animate-spin" /> : <Play />}
                  Start scan
                </Button>
              ) : (
                <Button size="lg" variant="destructive" onClick={handleScanStop} className="shrink-0">
                  <Square />
                  Stop
                </Button>
              )}
            </div>

            <div className="flex items-center gap-2 flex-wrap">
              {scanStatus && (
                <Badge variant="outline">
                  {scanStatus === 'running' && <Loader2 className="size-3 animate-spin" />}
                  {scanStatus}
                  {scanTotal !== null && ` · ${scanResults.length}/${scanTotal} done`}
                </Badge>
              )}
              {scanResults.length > 0 && (
                <Badge variant="secondary">
                  {scanResults.filter((r) => r.report.summary.vulnerable > 0).length} with signals
                </Badge>
              )}
            </div>

            {scanStartError && <div className="text-sm text-destructive">{scanStartError}</div>}
          </div>
        )}

        {mode !== 'scan' && (
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">Target request</h3>

          <div className="flex gap-2 items-end">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="sqli-method">Method</Label>
              <Select value={method} onValueChange={setMethod}>
                <SelectTrigger id="sqli-method" size="lg" className="w-28">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {METHODS.map((m) => (
                    <SelectItem key={m} value={m}>
                      {m}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5 flex-1">
              <Label htmlFor="sqli-url">URL (with the query parameters you want to test)</Label>
              <Input
                id="sqli-url"
                size="lg"
                className="font-mono"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://authorized-target.com/product?id=1"
              />
            </div>
            {mode === 'auto' && (
              <Button size="lg" onClick={handleScan} disabled={loading} className="shrink-0">
                {loading && <Loader2 className="animate-spin" />}
                Scan
              </Button>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sqli-body">Body (form-urlencoded, optional — e.g. user=admin&pass=123)</Label>
            <Textarea
              id="sqli-body"
              className="font-mono text-sm min-h-20 resize-y"
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="user=admin&pass=123"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label>Headers (optional — e.g. Authorization, Cookie, etc.)</Label>
            <HeadersEditor rows={headerRows} onChange={setHeaderRows} />
          </div>
        </div>
        )}

        {mode === 'auto' && (
          <>
            {error && <div className="text-sm text-destructive">{error}</div>}

            {report && (
              <>
                <Separator />
                <SqliReportView report={report} />
              </>
            )}
          </>
        )}

        {mode === 'manual' && (
          <>
            <div className="flex flex-col gap-4 border rounded-lg p-4">
              <h3 className="text-sm font-medium text-muted-foreground">Manual injection</h3>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="sqli-manual-param">Parameter to inject</Label>
                {params.length === 0 ? (
                  <p className="text-xs text-muted-foreground">
                    No parameters detected yet — add <code>?name=value</code> to the URL or fill
                    in the Body.
                  </p>
                ) : (
                  <Select value={manualParam ?? undefined} onValueChange={setManualParam}>
                    <SelectTrigger id="sqli-manual-param" size="lg" className="w-full">
                      <SelectValue placeholder="Choose a parameter" />
                    </SelectTrigger>
                    <SelectContent>
                      {params.map((p) => (
                        <SelectItem key={`${p.location}:${p.name}`} value={`${p.location}:${p.name}`}>
                          {p.name} <span className="text-muted-foreground">({p.location})</span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>

              <div className="flex gap-2 items-end">
                <div className="flex flex-col gap-1.5 flex-1">
                  <Label htmlFor="sqli-manual-payload">Payload (pick from the wordlist below or edit freely)</Label>
                  <Input
                    id="sqli-manual-payload"
                    size="lg"
                    className="font-mono"
                    value={manualPayload}
                    onChange={(e) => setManualPayload(e.target.value)}
                    placeholder="' OR 1=1--"
                  />
                </div>
                <Button
                  size="lg"
                  className="shrink-0"
                  onClick={handleManualSend}
                  disabled={manualSending || !manualParam || !manualPayload}
                >
                  {manualSending && <Loader2 className="animate-spin" />}
                  Send
                </Button>
              </div>
            </div>

            {manualError && <div className="text-sm text-destructive">{manualError}</div>}

            {manualResponse && (
              <div className="border rounded-lg p-4">
                <ResponseView
                  status={manualResponse.status}
                  headersJSON={manualResponse.headers}
                  body={manualResponse.body}
                />
              </div>
            )}

            <Separator />

            <div className="border rounded-lg min-w-0">
              <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium rounded-t-lg">
                Wordlist ({payloads.length} payloads)
              </div>
              <div className="divide-y max-h-96 overflow-auto">
                {payloads.length === 0 && (
                  <div className="px-4 py-3 text-sm text-muted-foreground">Loading...</div>
                )}
                {groupBy(payloads, (p) => p.type).map(([type, list]) => (
                  <div key={type}>
                    <div className="px-4 py-1.5 text-xs font-medium text-muted-foreground bg-muted/20">
                      {TYPE_LABEL[type] ?? type}
                    </div>
                    {list.map((p) => (
                      <div
                        key={p.value}
                        className={`flex items-center gap-3 px-4 py-2 hover:bg-muted/30 ${
                          manualPayload === p.value ? 'bg-muted/30' : ''
                        }`}
                      >
                        <span className="font-mono text-xs flex-1 min-w-0 break-all">
                          {p.value}
                        </span>
                        <Button
                          size="sm"
                          variant="outline"
                          className="shrink-0"
                          onClick={() => handleUsePayload(p.value)}
                        >
                          {copiedValue === p.value ? (
                            <>
                              <Check className="text-success" />
                              Copied
                            </>
                          ) : (
                            <>
                              <Copy />
                              Use
                            </>
                          )}
                        </Button>
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            </div>
          </>
        )}

        {mode === 'scan' && (
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium text-muted-foreground">Results</h3>
              {scanResults.length > 0 && (
                <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
                  <Switch checked={scanOnlyVulnerable} onCheckedChange={setScanOnlyVulnerable} />
                  Only show findings with signals
                </label>
              )}
            </div>
            {scanResults.length === 0 ? (
              <EmptyState
                icon={Bug}
                title="No results yet"
                description="Optionally set a host filter above and click Start scan."
              />
            ) : (
              <div className="flex flex-col gap-2">
                {[...scanResults]
                  .sort((a, b) => b.report.summary.vulnerable - a.report.summary.vulnerable)
                  .filter((item) => !scanOnlyVulnerable || item.report.summary.vulnerable > 0)
                  .map((item) => (
                    <SqliScanItemView key={item.history_id} item={item} onTest={handleOpenInChecker} />
                  ))}
                {scanOnlyVulnerable && scanResults.every((r) => r.report.summary.vulnerable === 0) && (
                  <p className="text-xs text-muted-foreground">No findings with signals yet.</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <SqliGuide />
      </aside>
    </div>
  )
}
