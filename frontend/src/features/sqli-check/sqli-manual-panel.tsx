import { useEffect, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ResponseView } from '@/components/response-view'
import { Loader2, Copy, Check } from 'lucide-react'
import { headerRowsToHeaders, type HeaderRow } from '@/components/headers-editor'
import { api } from '@/lib/api'
import { groupBy } from '@/lib/utils'
import { useClipboardCopy } from '@/lib/use-clipboard-copy'
import { detectParams } from '@/lib/detect-params'
import type { RepeaterResponse, SqliPayload } from '@/lib/types'

const TYPE_LABEL: Record<string, string> = {
  error: 'Error-based',
  boolean: 'Boolean-based',
  time: 'Time-based',
}

interface SqliManualPanelProps {
  method: string
  url: string
  body: string
  headerRows: HeaderRow[]
}

/** SQLi's "Manual" mode: pick a detected parameter, pick (or write) a
 * payload, and send it through the Repeater path — full control over
 * exactly one request at a time, as a follow-up to what Automatic found. */
export function SqliManualPanel({ method, url, body, headerRows }: SqliManualPanelProps) {
  const [payloads, setPayloads] = useState<SqliPayload[]>([])
  const { copiedValue, copy } = useClipboardCopy<string>()
  const [manualParam, setManualParam] = useState<string | null>(null)
  const [manualPayload, setManualPayload] = useState('')
  const [manualResponse, setManualResponse] = useState<RepeaterResponse | null>(null)
  const [manualSending, setManualSending] = useState(false)
  const [manualError, setManualError] = useState<string | null>(null)

  useEffect(() => {
    api.listSqliPayloads().then(setPayloads).catch(() => setPayloads([]))
  }, [])

  const params = detectParams(url, body)

  async function handleUsePayload(value: string) {
    setManualPayload(value)
    await copy(value, value)
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
    <>
      <div className="flex flex-col gap-4 border rounded-lg p-4">
        <h3 className="text-sm font-medium text-muted-foreground">Manual injection</h3>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="sqli-manual-param">Parameter to inject</Label>
          {params.length === 0 ? (
            <p className="text-xs text-muted-foreground">
              No parameters detected yet — add <code>?name=value</code> to the URL or fill in the
              Body.
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
                  <span className="font-mono text-xs flex-1 min-w-0 break-all">{p.value}</span>
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
  )
}
