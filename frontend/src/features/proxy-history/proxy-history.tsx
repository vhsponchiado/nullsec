import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { StatusDot } from '@/components/status-dot'
import { JsonView } from '@/components/json-view'
import { KeyValueTable, headersToPairs, queryParamsToPairs } from '@/components/key-value-table'
import { SendToMenu } from '@/components/send-to-menu'
import { parseJSONHeaders } from '@/lib/headers'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import type { HistoryItem, HistorySummary, SendTarget, SendableRequest } from '@/lib/types'
import { Search, Trash2, ChevronLeft, ChevronRight, Loader2 } from 'lucide-react'

const PAGE_SIZE = 200

function pathFromURL(url: string): string {
  try {
    const u = new URL(url)
    return u.pathname + u.search
  } catch {
    return url
  }
}

interface ProxyHistoryProps {
  onSendTo: (target: SendTarget, req: SendableRequest) => void
}

export function ProxyHistory({ onSendTo }: ProxyHistoryProps) {
  const [items, setItems] = useState<HistorySummary[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [loading, setLoading] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedDetail, setSelectedDetail] = useState<HistoryItem | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [query, setQuery] = useState('')

  const detailRequestId = useRef(0)

  async function load(opts?: { page?: number; host?: string }) {
    const targetPage = opts?.page ?? page
    setLoading(true)
    try {
      const data = await api.listHistory({
        host: (opts?.host ?? query) || undefined,
        page: targetPage,
        limit: PAGE_SIZE,
      })
      setItems(data.items ?? [])
      setTotal(data.total ?? 0)
      setPage(data.page ?? targetPage)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!selectedId) {
      setSelectedDetail(null)
      return
    }
    const requestId = ++detailRequestId.current
    setDetailLoading(true)
    api
      .getHistoryItem(selectedId)
      .then((item) => {
        if (detailRequestId.current === requestId) setSelectedDetail(item)
      })
      .catch(() => {
        if (detailRequestId.current === requestId) setSelectedDetail(null)
      })
      .finally(() => {
        if (detailRequestId.current === requestId) setDetailLoading(false)
      })
  }, [selectedId])

  // Matches the same "host contains query" filter the backend applies, so a
  // live-streamed item never bypasses whatever filter the user has active.
  function matchesFilter(item: HistorySummary) {
    return !query || item.host.toLowerCase().includes(query.toLowerCase())
  }

  // Only splice live events into the visible list while looking at the
  // first page (the "latest" page) — on any other page, an insert at the
  // front would silently shift every row out from under the user's place.
  // `item` is only ever a summary (the WS event doesn't carry headers/body),
  // so if the row happens to be the one currently open, re-fetch the full
  // detail rather than assuming the event has it.
  function upsert(item: HistorySummary) {
    if (!matchesFilter(item)) return
    if (page === 0) {
      setItems((prev) => [item, ...prev.filter((i) => i.id !== item.id)].slice(0, PAGE_SIZE))
    }
    setTotal((t) => t + (items.some((i) => i.id === item.id) ? 0 : 1))
    if (selectedId === item.id) {
      api.getHistoryItem(item.id).then(setSelectedDetail).catch(() => {})
    }
  }

  useProxyEvents((event) => {
    if (event.type === 'request_completed') {
      upsert(event.payload as HistorySummary)
    } else if (event.type === 'request_dropped') {
      const { id } = event.payload as { id: string }
      api.getHistoryItem(id).then(upsert).catch(() => {})
    }
  })

  async function handleClear() {
    if (!confirm('Clear the entire history? This action cannot be undone.')) return
    await api.clearHistory()
    setSelectedId(null)
    load({ page: 0 })
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className="flex flex-col gap-4 h-full">
      <div className="flex gap-2 items-center">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
          <Input
            size="lg"
            className="pl-10"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && load({ page: 0 })}
            placeholder="Search by host..."
          />
        </div>
        <Button size="lg" variant="outline" onClick={() => load({ page: 0 })}>
          Filter
        </Button>
        <div className="ml-auto flex items-center gap-2">
          <Button
            size="icon-lg"
            variant="outline"
            className="text-muted-foreground hover:text-destructive"
            onClick={handleClear}
            aria-label="Clear history"
          >
            <Trash2 />
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Badge variant="secondary">{total.toLocaleString()} total</Badge>
        {loading && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
        <div className="flex items-center gap-1 ml-auto">
          <Button
            size="icon-sm"
            variant="outline"
            disabled={page === 0 || loading}
            onClick={() => load({ page: page - 1 })}
            aria-label="Previous page"
          >
            <ChevronLeft />
          </Button>
          <span className="text-xs text-muted-foreground font-mono px-1">
            Page {page + 1} / {totalPages}
          </span>
          <Button
            size="icon-sm"
            variant="outline"
            disabled={page + 1 >= totalPages || loading}
            onClick={() => load({ page: page + 1 })}
            aria-label="Next page"
          >
            <ChevronRight />
          </Button>
        </div>
      </div>

      <div className="flex gap-4 flex-1 min-h-0">
        <div className="flex-1 min-w-0 border rounded-lg overflow-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Method</TableHead>
                <TableHead>Host</TableHead>
                <TableHead>Path</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Time</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <SendToMenu
                  key={item.id}
                  request={() => api.getHistoryItem(item.id)}
                  onSendTo={onSendTo}
                >
                  <TableRow
                    data-state={selectedId === item.id ? 'selected' : undefined}
                    className="cursor-pointer"
                    onClick={() => setSelectedId(item.id)}
                  >
                    <TableCell className="font-mono">{item.method}</TableCell>
                    <TableCell>{item.host}</TableCell>
                    <TableCell className="font-mono max-w-64 truncate">
                      {pathFromURL(item.url)}
                    </TableCell>
                    <TableCell>
                      {item.outcome === 'dropped' ? (
                        <Badge variant="outline">dropped</Badge>
                      ) : (
                        <StatusDot status={item.status} />
                      )}
                    </TableCell>
                    <TableCell>{item.size}</TableCell>
                    <TableCell>{new Date(item.created_at).toLocaleTimeString()}</TableCell>
                  </TableRow>
                </SendToMenu>
              ))}
              {items.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                    No requests captured yet.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        <AnimatePresence>
          {selectedId && (
          <motion.div
            key="detail-panel"
            initial={{ opacity: 0, x: 16 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 16 }}
            transition={{ duration: 0.18, ease: 'easeOut' }}
            className="w-[46%] shrink-0 flex flex-col gap-4"
          >
            {detailLoading || !selectedDetail ? (
              <div className="flex-1 border rounded-lg flex items-center justify-center text-sm text-muted-foreground gap-2">
                {detailLoading ? (
                  <>
                    <Loader2 className="size-4 animate-spin" />
                    Loading…
                  </>
                ) : (
                  'Could not load this request.'
                )}
              </div>
            ) : (
              <>
                <div className="border rounded-lg overflow-hidden flex-1 min-h-0 flex flex-col">
                  <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40">
                    <span className="font-mono text-sm">
                      <span className="font-semibold">{selectedDetail.method}</span>{' '}
                      {pathFromURL(selectedDetail.url)}
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs uppercase tracking-wide text-muted-foreground">
                        Request
                      </span>
                      <Button size="sm" variant="outline" onClick={() => onSendTo('repeater', selectedDetail)}>
                        Send to Repeater
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => onSendTo('sqli-check', selectedDetail)}
                      >
                        Send to SQLi
                      </Button>
                    </div>
                  </div>
                  <Tabs defaultValue="headers" className="flex-1 min-h-0 flex flex-col p-3 gap-3">
                    <TabsList variant="line" className="w-fit">
                      <TabsTrigger value="query">Query Params</TabsTrigger>
                      <TabsTrigger value="headers">Headers</TabsTrigger>
                      <TabsTrigger value="body">Body</TabsTrigger>
                    </TabsList>
                    <TabsContent value="query" className="flex-1 min-h-0 overflow-auto">
                      <KeyValueTable
                        pairs={queryParamsToPairs(selectedDetail.url)}
                        emptyLabel="No query params."
                      />
                    </TabsContent>
                    <TabsContent value="headers" className="flex-1 min-h-0 overflow-auto">
                      <KeyValueTable
                        pairs={headersToPairs(parseJSONHeaders(selectedDetail.headers))}
                        emptyLabel="No headers."
                      />
                    </TabsContent>
                    <TabsContent value="body" className="flex-1 min-h-0 overflow-auto">
                      <JsonView content={selectedDetail.body} />
                    </TabsContent>
                  </Tabs>
                </div>

                <div className="border rounded-lg overflow-hidden flex-1 min-h-0 flex flex-col">
                  <div className="flex items-center justify-between px-4 py-2.5 border-b bg-muted/40">
                    <StatusDot status={selectedDetail.status} />
                    <span className="text-xs uppercase tracking-wide text-muted-foreground">
                      Response
                    </span>
                  </div>
                  <Tabs defaultValue="body" className="flex-1 min-h-0 flex flex-col p-3 gap-3">
                    <TabsList variant="line" className="w-fit">
                      <TabsTrigger value="body">Body ({selectedDetail.size} bytes)</TabsTrigger>
                      <TabsTrigger value="headers">Headers</TabsTrigger>
                    </TabsList>
                    <TabsContent value="body" className="flex-1 min-h-0 overflow-auto">
                      <JsonView content={selectedDetail.resp_body} />
                    </TabsContent>
                    <TabsContent value="headers" className="flex-1 min-h-0 overflow-auto">
                      <KeyValueTable
                        pairs={headersToPairs(parseJSONHeaders(selectedDetail.resp_headers))}
                        emptyLabel="No headers."
                      />
                    </TabsContent>
                  </Tabs>
                </div>
              </>
            )}
          </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}
