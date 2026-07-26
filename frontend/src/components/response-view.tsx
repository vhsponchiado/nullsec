import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { JsonView } from '@/components/json-view'
import { headersToText, parseJSONHeaders } from '@/lib/headers'
import { Download } from 'lucide-react'

interface ResponseViewProps {
  status: number
  headersJSON: string
  body: string
  // When set, shows a "Download" button next to the Body label that saves
  // the raw response body as a .txt file under this name. Off by default —
  // only the callers that want it (Repeater) pass it in.
  downloadFileName?: string
}

function statusVariant(status: number): 'default' | 'secondary' | 'outline' {
  if (status === 0) return 'outline'
  if (status >= 500) return 'default'
  return 'secondary'
}

function downloadTextFile(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function ResponseView({ status, headersJSON, body, downloadFileName }: ResponseViewProps) {
  const headers = parseJSONHeaders(headersJSON)

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Label>Status</Label>
        <Badge variant={statusVariant(status)} className="text-sm">
          {status === 0 ? '—' : status}
        </Badge>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="resp-headers">Headers</Label>
        <Textarea
          id="resp-headers"
          className="font-mono text-sm min-h-32 resize-y"
          value={headersToText(headers)}
          readOnly
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between">
          <Label>Body</Label>
          {downloadFileName && (
            <Button
              size="sm"
              variant="outline"
              disabled={!body}
              onClick={() => downloadTextFile(downloadFileName, body)}
            >
              <Download />
              Download .txt
            </Button>
          )}
        </div>
        <div className="border rounded-lg max-h-96 overflow-auto bg-input/20">
          <JsonView content={body} />
        </div>
      </div>
    </div>
  )
}
