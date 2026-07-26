import { useState } from 'react'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { WandSparkles } from 'lucide-react'
import { JsonView, tryPrettyJSON } from '@/components/json-view'

interface JsonEditorProps {
  id?: string
  value: string
  onChange: (value: string) => void
  minHeightClass?: string
  readOnly?: boolean
}

/** An editable text area that shows syntax-highlighted, pretty-printed JSON
 * at rest and swaps to a plain editable textarea on click — full syntax
 * coloring while typing would need a real code-editor component, so this is
 * the "click to edit, highlighted when idle" middle ground. Includes a
 * one-click Format button that re-indents the current value if it parses as
 * JSON. */
export function JsonEditor({ id, value, onChange, minHeightClass = 'min-h-32', readOnly }: JsonEditorProps) {
  const [editing, setEditing] = useState(false)

  if (readOnly) {
    return (
      <div className={`rounded-lg border border-input overflow-auto ${minHeightClass}`}>
        <JsonView content={value} />
      </div>
    )
  }

  function handleFormat() {
    const pretty = tryPrettyJSON(value)
    if (pretty !== null) onChange(pretty)
  }

  if (editing) {
    return (
      <div className="relative">
        <Textarea
          id={id}
          autoFocus
          className={`font-mono text-sm resize-y pr-9 ${minHeightClass}`}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onBlur={() => setEditing(false)}
        />
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          className="absolute top-1.5 right-1.5 text-muted-foreground"
          onMouseDown={(e) => e.preventDefault()}
          onClick={handleFormat}
          aria-label="Format JSON"
          title="Format JSON"
        >
          <WandSparkles />
        </Button>
      </div>
    )
  }

  return (
    <div
      onClick={() => setEditing(true)}
      className={`rounded-lg border border-input overflow-auto cursor-text ${minHeightClass}`}
    >
      <JsonView content={value} emptyLabel="Click to edit..." />
    </div>
  )
}
