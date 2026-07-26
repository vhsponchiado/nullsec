import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ChevronDown, ChevronUp, SendHorizontal } from 'lucide-react'
import type { XssScanItemResult } from '@/lib/types'
import { XssReportView } from './xss-report-view'

export function XssScanItemView({
  item,
  onTest,
}: {
  item: XssScanItemResult
  onTest: (item: XssScanItemResult) => void
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
          <XssReportView report={item.report} />
        </div>
      )}
    </div>
  )
}
