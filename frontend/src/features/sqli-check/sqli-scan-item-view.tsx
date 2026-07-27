import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ChevronDown, ChevronUp, SendHorizontal } from 'lucide-react'
import type { SqliScanItemResult } from '@/lib/types'
import { SqliReportView } from './sqli-report-view'

export function SqliScanItemView({
  item,
  onTest,
}: {
  item: SqliScanItemResult
  onTest: (item: SqliScanItemResult) => void
}) {
  const { t } = useTranslation('sqli-check')
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
              {t('signal_count', { count: vulnerable })}
            </Badge>
          ) : (
            <Badge variant="secondary">{t('no_signal')}</Badge>
          )}
          <Button
            size="sm"
            variant="outline"
            onClick={() => onTest(item)}
            title={t('test_button_title')}
          >
            <SendHorizontal />
            {t('test_button')}
          </Button>
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-label={open ? t('collapse') : t('expand')}
          >
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
