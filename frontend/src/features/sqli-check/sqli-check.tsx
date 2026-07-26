import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Loader2 } from 'lucide-react'
import { headerRowsToHeaders, headersToHeaderRows, type HeaderRow } from '@/components/headers-editor'
import { api } from '@/lib/api'
import { textToHeaders, parseJSONHeaders } from '@/lib/headers'
import type { SqliReport, SqliScanItemResult } from '@/lib/types'
import type { RequestSeed } from '@/components/request-editor'
import { TargetRequestForm } from '@/components/target-request-form'
import { SqliGuide } from './sqli-check-guide'
import { SqliReportView } from './sqli-report-view'
import { SqliManualPanel } from './sqli-manual-panel'
import { SqliScanPanel } from './sqli-scan-panel'

interface SqliCheckProps {
  seed?: RequestSeed | null
}

export function SqliCheck({ seed }: SqliCheckProps) {
  const [mode, setMode] = useState<'auto' | 'manual' | 'scan'>('auto')

  // The in-flight target request — shared by Automatic and Manual mode
  // (both operate on the same URL/body/headers), set from a seed, typed in
  // directly, or loaded from a Scan result via "Test".
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

  // Automatic mode's own state — kept in the container (rather than its own
  // component) because its "Scan" button renders inside the shared target
  // form, next to the URL field.
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<SqliReport | null>(null)
  const [error, setError] = useState<string | null>(null)

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

  // Loads a scan result's original request (fetched fresh since ItemResult
  // only carries method/URL/report, not headers/body) into Automatic mode
  // for focused manual follow-up on that one finding.
  async function handleOpenInChecker(item: SqliScanItemResult) {
    const full = await api.getHistoryItem(item.history_id)
    setMethod(full.method)
    setUrl(full.url)
    setBody(full.body)
    setHeaderRows(headersToHeaderRows(parseJSONHeaders(full.headers)))
    setMode('auto')
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

        {mode === 'scan' && <SqliScanPanel onOpenInChecker={handleOpenInChecker} />}

        {mode !== 'scan' && (
          <TargetRequestForm
            idPrefix="sqli"
            method={method}
            onMethodChange={setMethod}
            url={url}
            onUrlChange={setUrl}
            urlPlaceholder="https://authorized-target.com/product?id=1"
            body={body}
            onBodyChange={setBody}
            bodyPlaceholder="user=admin&pass=123"
            headerRows={headerRows}
            onHeaderRowsChange={setHeaderRows}
            urlAction={
              mode === 'auto' ? (
                <Button size="lg" onClick={handleScan} disabled={loading} className="shrink-0">
                  {loading && <Loader2 className="animate-spin" />}
                  Scan
                </Button>
              ) : undefined
            }
          />
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
          <SqliManualPanel method={method} url={url} body={body} headerRows={headerRows} />
        )}
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <SqliGuide />
      </aside>
    </div>
  )
}
