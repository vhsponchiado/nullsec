import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { JsonEditor } from '@/components/json-editor'

export interface RequestEditorValue {
  method: string
  url: string
  headersText: string
  body: string
}

/** Shared "send this request to another tool" payload — used to seed the
 * Repeater, SQL Injection scanner, etc. from a History/Intercept entry. The
 * `nonce` changes on every send so the receiving feature can tell a fresh
 * seed apart from the same values sent twice in a row. */
export interface RequestSeed {
  value: RequestEditorValue
  nonce: number
}

interface RequestEditorProps {
  value: RequestEditorValue
  onChange: (value: RequestEditorValue) => void
  readOnly?: boolean
}

export function RequestEditor({ value, onChange, readOnly }: RequestEditorProps) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-3">
        <div className="flex flex-col gap-1.5 w-32">
          <Label htmlFor="req-method">Method</Label>
          <Input
            id="req-method"
            size="lg"
            value={value.method}
            readOnly={readOnly}
            onChange={(e) => onChange({ ...value, method: e.target.value })}
          />
        </div>
        <div className="flex flex-col gap-1.5 flex-1">
          <Label htmlFor="req-url">URL</Label>
          <Input
            id="req-url"
            size="lg"
            className="font-mono"
            value={value.url}
            readOnly={readOnly}
            onChange={(e) => onChange({ ...value, url: e.target.value })}
          />
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="req-headers">Headers</Label>
        <Textarea
          id="req-headers"
          className="font-mono text-sm min-h-32 resize-y"
          value={value.headersText}
          readOnly={readOnly}
          onChange={(e) => onChange({ ...value, headersText: e.target.value })}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="req-body">Body</Label>
        <JsonEditor
          id="req-body"
          minHeightClass="min-h-48"
          value={value.body}
          readOnly={readOnly}
          onChange={(body) => onChange({ ...value, body })}
        />
      </div>
    </div>
  )
}
