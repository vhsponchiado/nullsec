import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { JsonEditor } from '@/components/json-editor'
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
  HeadersEditor,
  headerRowsToHeaders,
  headersToHeaderRows,
  type HeaderRow,
} from '@/components/headers-editor'
import { ResponseView } from '@/components/response-view'
import { EmptyState } from '@/components/empty-state'
import { Send, Loader2, History, MousePointerClick } from 'lucide-react'
import { textToHeaders } from '@/lib/headers'
import { api } from '@/lib/api'
import type { RepeaterResponse } from '@/lib/types'
import type { RequestSeed } from '@/components/request-editor'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

export type RepeaterSeed = RequestSeed

interface RequestForm {
  method: string
  url: string
  body: string
  headerRows: HeaderRow[]
}

interface Attempt {
  id: string
  request: RequestForm
  response: RepeaterResponse | null
  sending: boolean
  error: string | null
}

const EMPTY_FORM: RequestForm = { method: 'GET', url: '', body: '', headerRows: [{ key: '', value: '' }] }

function responseFileName(url: string): string {
  let base = 'response'
  try {
    const u = new URL(url)
    base = `${u.hostname}${u.pathname}`.replace(/\/+$/, '').replace(/[^a-zA-Z0-9._-]+/g, '-')
  } catch {
    // not a valid URL yet, so fall back to the generic name
  }
  return `${base || 'response'}.txt`
}

interface RepeaterProps {
  seed: RepeaterSeed | null
}

export function Repeater({ seed }: RepeaterProps) {
  const { t } = useTranslation('repeater')
  const [form, setForm] = useState<RequestForm>(EMPTY_FORM)
  const [attempts, setAttempts] = useState<Attempt[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const lastNonce = useRef<number | null>(null)

  useEffect(() => {
    if (seed && seed.nonce !== lastNonce.current) {
      lastNonce.current = seed.nonce
      setForm({
        method: seed.value.method,
        url: seed.value.url,
        body: seed.value.body,
        headerRows: headersToHeaderRows(textToHeaders(seed.value.headersText)),
      })
      setSelectedId(null)
    }
  }, [seed])

  async function handleSend() {
    const id = crypto.randomUUID()
    const attempt: Attempt = { id, request: form, response: null, sending: true, error: null }
    setAttempts((prev) => [attempt, ...prev])
    setSelectedId(id)

    try {
      const response = await api.repeaterSend({
        method: form.method,
        url: form.url,
        headers: headerRowsToHeaders(form.headerRows),
        body: form.body,
      })
      setAttempts((prev) =>
        prev.map((a) => (a.id === id ? { ...a, response, sending: false, error: response.error ?? null } : a)),
      )
    } catch (err) {
      setAttempts((prev) =>
        prev.map((a) => (a.id === id ? { ...a, sending: false, error: (err as Error).message } : a)),
      )
    }
  }

  const selected = attempts.find((a) => a.id === selectedId) ?? null

  return (
    <div className="flex gap-4 h-full min-h-0 min-w-0">
      <div className="flex-1 min-w-0 flex flex-col gap-4 overflow-auto pr-1">
        <div className="flex flex-col gap-4 border rounded-lg p-4">
          <h3 className="text-sm font-medium text-muted-foreground">{t('request_title')}</h3>

          <div className="flex gap-2 items-end">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="repeater-method">{t('method_label')}</Label>
              <Select value={form.method} onValueChange={(v) => setForm({ ...form, method: v })}>
                <SelectTrigger id="repeater-method" size="lg" className="w-28">
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
              <Label htmlFor="repeater-url">{t('url_label')}</Label>
              <Input
                id="repeater-url"
                size="lg"
                className="font-mono"
                value={form.url}
                onChange={(e) => setForm({ ...form, url: e.target.value })}
                placeholder={t('url_placeholder')}
              />
            </div>
            <Button size="lg" className="shrink-0" onClick={handleSend}>
              <Send />
              {t('send')}
            </Button>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="repeater-body">{t('body_label')}</Label>
            <JsonEditor
              id="repeater-body"
              minHeightClass="min-h-24"
              value={form.body}
              onChange={(body) => setForm({ ...form, body })}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label>{t('headers_label')}</Label>
            <HeadersEditor
              rows={form.headerRows}
              onChange={(rows) => setForm({ ...form, headerRows: rows })}
            />
          </div>
        </div>

        <div className="border rounded-lg min-w-0 overflow-hidden">
          <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium">
            {t('send_history_title')}
          </div>
          {attempts.length === 0 ? (
            <EmptyState
              icon={History}
              title={t('empty_sends_title')}
              description={t('empty_sends_description')}
            />
          ) : (
            <div className="divide-y max-h-64 overflow-auto">
              {attempts.map((a) => (
                <button
                  key={a.id}
                  onClick={() => {
                    setSelectedId(a.id)
                    setForm(a.request)
                  }}
                  className={`w-full text-left px-4 py-2.5 hover:bg-muted/30 cursor-pointer transition-colors ${
                    selectedId === a.id ? 'bg-muted/30' : ''
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm font-medium">{a.request.method}</span>
                    {a.sending ? (
                      <Badge variant="outline">{t('sending_badge')}</Badge>
                    ) : a.error ? (
                      <Badge variant="outline">{t('error_badge')}</Badge>
                    ) : (
                      <Badge variant="secondary">{a.response?.status}</Badge>
                    )}
                  </div>
                  <div className="text-xs text-muted-foreground truncate font-mono">
                    {a.request.url}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="flex-1 min-w-0 border rounded-lg overflow-hidden flex flex-col">
        <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium">{t('response_title')}</div>
        <div className="flex-1 overflow-auto p-4">
          {selected ? (
            selected.sending ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                {t('sending_label')}
              </div>
            ) : selected.error ? (
              <div className="text-sm text-destructive">{t('error_prefix', { error: selected.error })}</div>
            ) : selected.response ? (
              <ResponseView
                status={selected.response.status}
                headersJSON={selected.response.headers}
                body={selected.response.body}
                downloadFileName={responseFileName(selected.request.url)}
              />
            ) : null
          ) : (
            <EmptyState
              icon={MousePointerClick}
              title={t('empty_response_title')}
              description={t('empty_response_description')}
            />
          )}
        </div>
      </div>
    </div>
  )
}
