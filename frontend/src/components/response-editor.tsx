import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { JsonEditor } from '@/components/json-editor'

export interface ResponseEditorValue {
  status: number
  headersText: string
  body: string
}

interface ResponseEditorProps {
  value: ResponseEditorValue
  onChange: (value: ResponseEditorValue) => void
}

export function ResponseEditor({ value, onChange }: ResponseEditorProps) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5 w-32">
        <Label htmlFor="resp-status">Status</Label>
        <Input
          id="resp-status"
          type="number"
          size="lg"
          value={value.status}
          onChange={(e) => onChange({ ...value, status: Number(e.target.value) || 0 })}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="resp-edit-headers">Headers</Label>
        <Textarea
          id="resp-edit-headers"
          className="font-mono text-sm min-h-32 resize-y"
          value={value.headersText}
          onChange={(e) => onChange({ ...value, headersText: e.target.value })}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="resp-edit-body">Body</Label>
        <JsonEditor
          id="resp-edit-body"
          minHeightClass="min-h-48"
          value={value.body}
          onChange={(body) => onChange({ ...value, body })}
        />
      </div>
    </div>
  )
}
