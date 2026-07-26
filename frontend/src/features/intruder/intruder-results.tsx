import { useCallback, useState } from 'react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { EmptyState } from '@/components/empty-state'
import { ResponseView } from '@/components/response-view'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ArrowUpDown, Check, Crosshair, Loader2 } from 'lucide-react'
import { useSortedResults } from '@/lib/use-sorted-results'
import type { IntruderResultDetail, IntruderResultSummary } from '@/lib/types'

type SortKey = 'index' | 'status' | 'length' | 'duration_ms'

interface IntruderResultsProps {
  results: IntruderResultSummary[]
  positionLabels: string[]
  selectedIndex: number | null
  onSelectRow: (index: number) => void
  hasGrep: boolean
}

function statusVariant(status: number): 'default' | 'secondary' | 'outline' {
  if (status === 0) return 'outline'
  if (status >= 500) return 'default'
  return 'secondary'
}

export function IntruderResults({
  results,
  positionLabels,
  selectedIndex,
  onSelectRow,
  hasGrep,
}: IntruderResultsProps) {
  const [sortKey, setSortKey] = useState<SortKey>('index')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const compare = useCallback(
    (a: IntruderResultSummary, b: IntruderResultSummary) => {
      const av = sortKey === 'duration_ms' ? a.duration_ms : a[sortKey]
      const bv = sortKey === 'duration_ms' ? b.duration_ms : b[sortKey]
      return sortDir === 'asc' ? av - bv : bv - av
    },
    [sortKey, sortDir],
  )
  const sorted = useSortedResults(results, compare)

  function toggleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  if (results.length === 0) {
    return (
      <EmptyState
        icon={Crosshair}
        title="No results yet"
        description="Configure the attack above and click Start attack."
      />
    )
  }

  return (
    <div className="border rounded-lg overflow-auto max-h-[28rem]">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('index')}>
              # <ArrowUpDown className="inline size-3" />
            </TableHead>
            {positionLabels.map((label, i) => (
              <TableHead key={i}>{label}</TableHead>
            ))}
            <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('status')}>
              Status <ArrowUpDown className="inline size-3" />
            </TableHead>
            <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('length')}>
              Length <ArrowUpDown className="inline size-3" />
            </TableHead>
            <TableHead className="cursor-pointer select-none" onClick={() => toggleSort('duration_ms')}>
              Time <ArrowUpDown className="inline size-3" />
            </TableHead>
            {hasGrep && <TableHead>Match</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((r) => (
            <TableRow
              key={r.index}
              onClick={() => onSelectRow(r.index)}
              className={`cursor-pointer ${selectedIndex === r.index ? 'bg-muted/50' : ''}`}
            >
              <TableCell className="font-mono text-xs">{r.index}</TableCell>
              {positionLabels.map((_, i) => (
                <TableCell key={i} className="font-mono text-xs max-w-48 truncate">
                  {r.payloads[i] ?? ''}
                </TableCell>
              ))}
              <TableCell>
                {r.error ? (
                  <Badge variant="outline">error</Badge>
                ) : (
                  <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
                )}
              </TableCell>
              <TableCell className="font-mono text-xs">{r.length}</TableCell>
              <TableCell className="font-mono text-xs">{r.duration_ms}ms</TableCell>
              {hasGrep && (
                <TableCell>{r.match && <Check className="size-4 text-success" />}</TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

interface IntruderResultDetailViewProps {
  detail: IntruderResultDetail | null
  loading: boolean
}

export function IntruderResultDetailView({ detail, loading }: IntruderResultDetailViewProps) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground p-4">
        <Loader2 className="size-4 animate-spin" />
        Loading…
      </div>
    )
  }

  if (!detail) {
    return (
      <EmptyState
        icon={Crosshair}
        title="Nothing selected"
        description="Click a row to see the exact request sent and the response received."
      />
    )
  }

  if (detail.error) {
    return <div className="text-sm text-destructive p-4">Error: {detail.error}</div>
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 p-4">
      <div className="flex flex-col gap-3 min-w-0">
        <h4 className="text-sm font-medium text-muted-foreground">
          Request ({detail.request_method})
        </h4>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="intruder-detail-url" className="text-xs">URL</Label>
          <Textarea id="intruder-detail-url" readOnly className="font-mono text-xs min-h-16 resize-y" value={detail.request_url} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="intruder-detail-headers" className="text-xs">Headers</Label>
          <Textarea
            id="intruder-detail-headers"
            readOnly
            className="font-mono text-xs min-h-24 resize-y"
            value={formatHeadersJSON(detail.request_headers)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="intruder-detail-body" className="text-xs">Body</Label>
          <Textarea id="intruder-detail-body" readOnly className="font-mono text-xs min-h-24 resize-y" value={detail.request_body} />
        </div>
      </div>

      <div className="min-w-0">
        <h4 className="text-sm font-medium text-muted-foreground mb-3">Response</h4>
        <ResponseView status={detail.status} headersJSON={detail.response_headers} body={detail.response_body} />
      </div>
    </div>
  )
}

function formatHeadersJSON(raw: string): string {
  try {
    const parsed = JSON.parse(raw) as Record<string, string[]>
    return Object.entries(parsed)
      .flatMap(([key, values]) => values.map((v) => `${key}: ${v}`))
      .join('\n')
  } catch {
    return raw
  }
}
