import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { RequestEditor, type RequestEditorValue } from '@/components/request-editor'
import { ResponseEditor, type ResponseEditorValue } from '@/components/response-editor'
import { SendToMenu } from '@/components/send-to-menu'
import { EmptyState } from '@/components/empty-state'
import { Inbox, MousePointerClick, Radio } from 'lucide-react'
import { headersToText, textToHeaders } from '@/lib/headers'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import type { PendingRequest, PendingResponse, SendTarget, SendableRequest } from '@/lib/types'

type Item =
  | { kind: 'request'; data: PendingRequest }
  | { kind: 'response'; data: PendingResponse }

function itemKey(item: Item) {
  return `${item.kind}:${item.data.id}`
}

function toRequestEditorValue(req: PendingRequest): RequestEditorValue {
  return {
    method: req.method,
    url: req.url,
    headersText: headersToText(req.headers),
    body: req.body,
  }
}

function toResponseEditorValue(resp: PendingResponse): ResponseEditorValue {
  return {
    status: resp.status,
    headersText: headersToText(resp.headers),
    body: resp.body,
  }
}

interface InterceptProps {
  onSendTo: (target: SendTarget, req: SendableRequest) => void
}

export function Intercept({ onSendTo }: InterceptProps) {
  const [enabled, setEnabled] = useState(false)
  const [requests, setRequests] = useState<PendingRequest[]>([])
  const [responses, setResponses] = useState<PendingResponse[]>([])
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [requestValue, setRequestValue] = useState<RequestEditorValue | null>(null)
  const [responseValue, setResponseValue] = useState<ResponseEditorValue | null>(null)

  useEffect(() => {
    Promise.all([api.getInterceptStatus(), api.getResponseInterceptStatus()]).then(
      ([reqStatus, respStatus]) => setEnabled(reqStatus.enabled || respStatus.enabled),
    )
    api.listPending().then((list) => setRequests(list ?? []))
    api.listPendingResponses().then((list) => setResponses(list ?? []))
  }, [])

  useProxyEvents((event) => {
    if (event.type === 'request_intercepted') {
      setRequests((prev) => [...prev, event.payload as PendingRequest])
    } else if (event.type === 'request_forwarded' || event.type === 'request_dropped') {
      const { id } = event.payload as { id: string }
      setRequests((prev) => prev.filter((p) => p.id !== id))
      setSelectedKey((cur) => (cur === `request:${id}` ? null : cur))
    } else if (event.type === 'response_intercepted') {
      setResponses((prev) => [...prev, event.payload as PendingResponse])
    } else if (event.type === 'response_forwarded' || event.type === 'response_dropped') {
      const { id } = event.payload as { id: string }
      setResponses((prev) => prev.filter((p) => p.id !== id))
      setSelectedKey((cur) => (cur === `response:${id}` ? null : cur))
    }
  })

  const items: Item[] = [
    ...requests.map((data): Item => ({ kind: 'request', data })),
    ...responses.map((data): Item => ({ kind: 'response', data })),
  ]
  const selected = items.find((item) => itemKey(item) === selectedKey) ?? null

  useEffect(() => {
    if (!selected) {
      setRequestValue(null)
      setResponseValue(null)
      return
    }
    if (selected.kind === 'request') {
      setRequestValue(toRequestEditorValue(selected.data))
      setResponseValue(null)
    } else {
      setResponseValue(toResponseEditorValue(selected.data))
      setRequestValue(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedKey])

  async function handleToggle(checked: boolean) {
    const [reqStatus, respStatus] = await Promise.all([
      api.toggleIntercept(checked),
      api.toggleResponseIntercept(checked),
    ])
    setEnabled(reqStatus.enabled || respStatus.enabled)
  }

  async function handleForward() {
    if (!selected) return
    if (selected.kind === 'request' && requestValue) {
      await api.forwardIntercepted(selected.data.id, {
        method: requestValue.method,
        url: requestValue.url,
        headers: textToHeaders(requestValue.headersText),
        body: requestValue.body,
      })
      setRequests((prev) => prev.filter((p) => p.id !== selected.data.id))
    } else if (selected.kind === 'response' && responseValue) {
      await api.forwardInterceptedResponse(selected.data.id, {
        status: responseValue.status,
        headers: textToHeaders(responseValue.headersText),
        body: responseValue.body,
      })
      setResponses((prev) => prev.filter((p) => p.id !== selected.data.id))
    }
    setSelectedKey(null)
  }

  async function handleDrop() {
    if (!selected) return
    if (selected.kind === 'request') {
      await api.dropIntercepted(selected.data.id)
      setRequests((prev) => prev.filter((p) => p.id !== selected.data.id))
    } else {
      await api.dropInterceptedResponse(selected.data.id)
      setResponses((prev) => prev.filter((p) => p.id !== selected.data.id))
    }
    setSelectedKey(null)
  }

  async function handleForwardAll() {
    const requestIds = requests.map((p) => p.id)
    const responseIds = responses.map((p) => p.id)
    setSelectedKey(null)
    setRequests([])
    setResponses([])
    await Promise.all([
      // Empty edit payload — the backend falls back to each pending
      // request's own original method/url/headers/body when fields are omitted.
      ...requestIds.map((id) =>
        api.forwardIntercepted(id, { method: '', url: '', headers: {}, body: '' }).catch(() => {
          /* already resolved or failed — ignore, WS will reconcile state */
        }),
      ),
      ...responseIds.map((id) =>
        api.forwardInterceptedResponse(id, { status: 0, headers: {}, body: '' }).catch(() => {
          /* already resolved or failed — ignore, WS will reconcile state */
        }),
      ),
    ])
  }

  return (
    <div className="flex flex-col gap-4 h-full">
      <div className="flex items-center gap-3 border rounded-lg px-4 py-3 bg-muted/20">
        <Switch id="intercept-toggle" checked={enabled} onCheckedChange={handleToggle} />
        <Label htmlFor="intercept-toggle" className="text-base flex items-center gap-2">
          <Radio className={`size-4 ${enabled ? 'text-primary' : 'text-muted-foreground'}`} />
          Intercept {enabled ? 'on' : 'off'}
        </Label>
        <Badge variant={items.length > 0 ? 'secondary' : 'outline'}>{items.length} pending</Badge>
        {items.length > 0 && (
          <Button size="sm" variant="outline" className="ml-auto" onClick={handleForwardAll}>
            Forward all
          </Button>
        )}
      </div>

      <div className="flex gap-4 flex-1 min-h-0">
        <div className="w-72 shrink-0 border rounded-lg overflow-hidden flex flex-col">
          <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium">Pending</div>
          <div className="flex-1 overflow-auto">
            {items.length === 0 && (
              <EmptyState
                icon={Inbox}
                title="Nothing held"
                description="Turn on intercept and browse through the proxy."
              />
            )}
            {items.map((item) => {
              const key = itemKey(item)
              return (
                <InterceptItemRow
                  key={key}
                  item={item}
                  selected={selectedKey === key}
                  onSelect={() => setSelectedKey(key)}
                  onSendTo={onSendTo}
                />
              )
            })}
          </div>
        </div>

        <div className="flex-1 min-w-0 border rounded-lg overflow-hidden flex flex-col">
          <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium">
            {selected?.kind === 'response' ? 'Response' : 'Request'}
          </div>
          <div className="flex-1 overflow-auto p-4">
            <AnimatePresence mode="wait" initial={false}>
              {selected?.kind === 'request' && requestValue ? (
                <motion.div
                  key={`request-${selectedKey}`}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  className="flex flex-col gap-4"
                >
                  <RequestEditor value={requestValue} onChange={setRequestValue} />
                  <div className="flex gap-3">
                    <Button size="lg" onClick={handleForward}>
                      Forward
                    </Button>
                    <Button size="lg" variant="outline" onClick={handleDrop}>
                      Drop
                    </Button>
                  </div>
                </motion.div>
              ) : selected?.kind === 'response' && responseValue ? (
                <motion.div
                  key={`response-${selectedKey}`}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  className="flex flex-col gap-4"
                >
                  <div className="text-xs text-muted-foreground font-mono truncate">
                    {selected.data.request_method} {selected.data.request_url}
                  </div>
                  <ResponseEditor value={responseValue} onChange={setResponseValue} />
                  <div className="flex gap-3">
                    <Button size="lg" onClick={handleForward}>
                      Forward
                    </Button>
                    <Button size="lg" variant="outline" onClick={handleDrop}>
                      Drop
                    </Button>
                  </div>
                </motion.div>
              ) : (
                <motion.div
                  key="empty"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                >
                  <EmptyState
                    icon={MousePointerClick}
                    title="Nothing selected"
                    description="Select a pending item to edit and release."
                  />
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>
      </div>
    </div>
  )
}

function InterceptItemRow({
  item,
  selected,
  onSelect,
  onSendTo,
}: {
  item: Item
  selected: boolean
  onSelect: () => void
  onSendTo: (target: SendTarget, req: SendableRequest) => void
}) {
  const row = (
    <button
      onClick={onSelect}
      className={`w-full text-left px-4 py-3 border-b last:border-b-0 hover:bg-accent transition-colors cursor-pointer ${
        selected ? 'bg-accent' : ''
      }`}
    >
      <div className="flex items-center gap-2">
        <Badge variant={item.kind === 'response' ? 'secondary' : 'outline'}>
          {item.kind === 'response' ? 'Response' : 'Request'}
        </Badge>
        <span className="font-mono text-sm font-medium">
          {item.kind === 'request' ? item.data.method : item.data.request_method}
        </span>
        {item.kind === 'response' && <Badge variant="outline">{item.data.status}</Badge>}
      </div>
      <div className="text-xs text-muted-foreground truncate">
        {item.kind === 'request' ? item.data.url : item.data.request_url}
      </div>
    </button>
  )

  if (item.kind === 'request') {
    return (
      <SendToMenu request={item.data} onSendTo={onSendTo} allowInterceptResponse={false}>
        {row}
      </SendToMenu>
    )
  }
  return row
}
