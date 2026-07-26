import { useRef } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { X, Plus, Globe } from 'lucide-react'
import { MarkButton } from '@/components/mark-button'
import type { Headers } from '@/lib/types'

export interface HeaderRow {
  key: string
  value: string
}

// Go's http.Client sends "User-Agent: Go-http-client/1.1" when none is set,
// a well-known automation signature that WAFs (Cloudflare's in particular)
// block on sight — often before the request ever reaches the target's own
// login logic. This preset fills in the headers a real browser would send,
// which clears that specific bot-signature check. It can't spoof the TLS/JA3
// fingerprint, so a WAF checking that layer will still need other handling.
const BROWSER_PRESET: HeaderRow[] = [
  {
    key: 'User-Agent',
    value:
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
  },
  { key: 'Accept', value: 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8' },
  { key: 'Accept-Language', value: 'pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7' },
]

function applyPreset(rows: HeaderRow[], preset: HeaderRow[]): HeaderRow[] {
  const next = rows.filter((r) => r.key.trim() || r.value.trim())
  for (const p of preset) {
    const idx = next.findIndex((r) => r.key.trim().toLowerCase() === p.key.toLowerCase())
    if (idx >= 0) next[idx] = { ...next[idx], value: p.value }
    else next.push({ ...p })
  }
  return next
}

interface HeadersEditorProps {
  rows: HeaderRow[]
  onChange: (rows: HeaderRow[]) => void
  // Intruder's request builder needs to mark §…§ payload positions inside
  // header values — everywhere else this stays off and renders unchanged.
  markable?: boolean
}

export function HeadersEditor({ rows, onChange, markable }: HeadersEditorProps) {
  const valueEls = useRef<Map<number, HTMLInputElement>>(new Map())

  function updateRow(i: number, patch: Partial<HeaderRow>) {
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  }

  function removeRow(i: number) {
    onChange(rows.filter((_, idx) => idx !== i))
  }

  function addRow() {
    onChange([...rows, { key: '', value: '' }])
  }

  return (
    <div className="flex flex-col gap-2">
      {rows.map((row, i) => (
        <div key={i} className="flex gap-2 items-center">
          <Input
            placeholder="Header"
            value={row.key}
            onChange={(e) => updateRow(i, { key: e.target.value })}
            className="flex-1 font-mono"
          />
          <Input
            ref={(el) => {
              if (el) valueEls.current.set(i, el)
              else valueEls.current.delete(i)
            }}
            placeholder="Value"
            value={row.value}
            onChange={(e) => updateRow(i, { value: e.target.value })}
            className="flex-1 font-mono"
          />
          {markable && (
            <MarkButton
              className="shrink-0"
              getEl={() => valueEls.current.get(i) ?? null}
              value={row.value}
              onChange={(v) => updateRow(i, { value: v })}
            />
          )}
          <Button
            size="icon"
            variant="ghost"
            className="shrink-0 text-muted-foreground hover:text-destructive"
            onClick={() => removeRow(i)}
            aria-label="Remove header"
          >
            <X />
          </Button>
        </div>
      ))}
      <div className="flex gap-2">
        <Button size="sm" variant="outline" onClick={addRow} className="w-fit">
          <Plus />
          Add header
        </Button>
        <Button
          size="sm"
          variant="outline"
          className="w-fit"
          onClick={() => onChange(applyPreset(rows, BROWSER_PRESET))}
          title="Fill in User-Agent, Accept and Accept-Language as a real browser would send them — helps get past WAF rules that block Go's default User-Agent"
        >
          <Globe />
          Browser headers
        </Button>
      </div>
    </div>
  )
}

export function headerRowsToHeaders(rows: HeaderRow[]): Headers {
  const out: Headers = {}
  for (const row of rows) {
    const key = row.key.trim()
    if (!key || !row.value) continue
    if (!out[key]) out[key] = []
    out[key].push(row.value)
  }
  return out
}

export function headersToHeaderRows(headers: Headers | undefined): HeaderRow[] {
  const rows: HeaderRow[] = []
  if (headers) {
    for (const [key, values] of Object.entries(headers)) {
      for (const value of values) rows.push({ key, value })
    }
  }
  return rows.length > 0 ? rows : [{ key: '', value: '' }]
}
