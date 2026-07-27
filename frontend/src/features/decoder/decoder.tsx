import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Copy, Check, X, Loader2, RotateCcw, Sparkles } from 'lucide-react'
import { api } from '@/lib/api'
import { useClipboardCopy } from '@/lib/use-clipboard-copy'
import type { DecodeOperation } from '@/lib/types'
import { DecoderGuide } from './decoder-guide'

interface Panel {
  id: number
  label: string
  content: string
  view: 'text' | 'hex'
}

function toHexDump(s: string): string {
  const bytes = new TextEncoder().encode(s)
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join(' ')
}

let nextPanelId = 1

function makeInputPanel(label: string): Panel {
  return { id: 0, label, content: '', view: 'text' }
}

export function Decoder() {
  const { t } = useTranslation('decoder')
  const [operations, setOperations] = useState<DecodeOperation[]>([])
  const [panels, setPanels] = useState<Panel[]>([makeInputPanel(t('input_label'))])
  const [busyIndex, setBusyIndex] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { copiedValue: copiedId, copy } = useClipboardCopy<number>()

  useEffect(() => {
    api.listDecodeOperations().then(setOperations).catch(() => setOperations([]))
  }, [])

  const reversibleOps = operations.filter((op) => !op.hash_only)
  const hashOps = operations.filter((op) => op.hash_only)

  function updateContent(index: number, content: string) {
    setPanels((prev) => {
      const next = prev.slice(0, index + 1)
      next[index] = { ...next[index], content }
      return next
    })
  }

  function toggleView(index: number, view: 'text' | 'hex') {
    setPanels((prev) => prev.map((p, i) => (i === index ? { ...p, view } : p)))
  }

  function truncateAfter(index: number) {
    setPanels((prev) => prev.slice(0, index + 1))
  }

  async function applyTransform(index: number, op: DecodeOperation, mode: 'encode' | 'decode') {
    const panel = panels[index]
    if (!panel.content) return
    setBusyIndex(index)
    setError(null)
    try {
      const result = await api.decode(op.key, mode, panel.content)
      if (result.error) {
        setError(result.error)
        return
      }
      const label = op.hash_only ? op.label : `${op.label} ${mode}`
      setPanels((prev) => [
        ...prev.slice(0, index + 1),
        { id: nextPanelId++, label, content: result.output ?? '', view: 'text' },
      ])
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusyIndex(null)
    }
  }

  async function applySmart(index: number) {
    const panel = panels[index]
    if (!panel.content) return
    setBusyIndex(index)
    setError(null)
    try {
      const steps = (await api.smartDecode(panel.content)).steps ?? []
      if (steps.length === 0) {
        setError(t('smart_decode_error'))
        return
      }
      const newPanels = steps.map((s) => ({
        id: nextPanelId++,
        label: s.label,
        content: s.output,
        view: 'text' as const,
      }))
      setPanels((prev) => [...prev.slice(0, index + 1), ...newPanels])
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusyIndex(null)
    }
  }

  function reset() {
    setPanels([makeInputPanel(t('input_label'))])
    setError(null)
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-4 overflow-auto pr-1">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-muted-foreground">
            {t('panel_count', { count: panels.length })}
          </h3>
          {panels.length > 1 && (
            <Button size="sm" variant="ghost" onClick={reset} className="text-muted-foreground">
              <RotateCcw />
              {t('reset')}
            </Button>
          )}
        </div>

        {error && <div className="text-sm text-destructive">{error}</div>}

        {panels.map((panel, index) => (
          <DecoderPanelRow
            key={panel.id}
            panel={panel}
            index={index}
            busy={busyIndex === index}
            disabled={busyIndex !== null}
            reversibleOps={reversibleOps}
            hashOps={hashOps}
            copied={copiedId === panel.id}
            removable={index > 0}
            onContentChange={(v) => updateContent(index, v)}
            onViewChange={(v) => toggleView(index, v)}
            onTransform={(op, mode) => applyTransform(index, op, mode)}
            onSmart={() => applySmart(index)}
            onCopy={() => copy(panel.id, panel.content)}
            onRemove={() => truncateAfter(index - 1)}
          />
        ))}
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <DecoderGuide />
      </aside>
    </div>
  )
}

interface DecoderPanelRowProps {
  panel: Panel
  index: number
  busy: boolean
  disabled: boolean
  reversibleOps: DecodeOperation[]
  hashOps: DecodeOperation[]
  copied: boolean
  removable: boolean
  onContentChange: (value: string) => void
  onViewChange: (view: 'text' | 'hex') => void
  onTransform: (op: DecodeOperation, mode: 'encode' | 'decode') => void
  onSmart: () => void
  onCopy: () => void
  onRemove: () => void
}

function DecoderPanelRow({
  panel,
  busy,
  disabled,
  reversibleOps,
  hashOps,
  copied,
  removable,
  onContentChange,
  onViewChange,
  onTransform,
  onSmart,
  onCopy,
  onRemove,
}: DecoderPanelRowProps) {
  const { t } = useTranslation('decoder')
  const textRef = useRef<HTMLTextAreaElement>(null)
  const displayValue = panel.view === 'hex' ? toHexDump(panel.content) : panel.content

  return (
    <div className="border rounded-lg p-3 flex gap-3 min-w-0">
      <div className="flex flex-col gap-1.5 flex-1 min-w-0">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">{panel.label}</span>
          <div className="flex items-center gap-1">
            <Button
              size="icon-sm"
              variant="ghost"
              disabled={!panel.content}
              onClick={onCopy}
              aria-label={copied ? t('copied_aria') : t('copy_aria')}
            >
              {copied ? <Check className="text-success" /> : <Copy />}
            </Button>
            {removable && (
              <Button
                size="icon-sm"
                variant="ghost"
                onClick={onRemove}
                aria-label={t('remove_panel_aria')}
              >
                <X />
              </Button>
            )}
          </div>
        </div>
        <Textarea
          ref={textRef}
          className="font-mono text-sm min-h-24 resize-y"
          value={displayValue}
          readOnly={panel.view === 'hex'}
          onChange={(e) => onContentChange(e.target.value)}
          placeholder={t('input_placeholder')}
        />
      </div>

      <div className="flex flex-col gap-1.5 w-44 shrink-0">
        <div className="flex rounded-md border overflow-hidden text-xs">
          <button
            onClick={() => onViewChange('text')}
            className={`flex-1 py-1 cursor-pointer ${panel.view === 'text' ? 'bg-muted font-medium' : 'text-muted-foreground'}`}
          >
            {t('view_text')}
          </button>
          <button
            onClick={() => onViewChange('hex')}
            className={`flex-1 py-1 cursor-pointer border-l ${panel.view === 'hex' ? 'bg-muted font-medium' : 'text-muted-foreground'}`}
          >
            {t('view_hex')}
          </button>
        </div>

        <Select
          value=""
          disabled={disabled || !panel.content}
          onValueChange={(v) => {
            const op = reversibleOps.find((o) => o.key === v)
            if (op) onTransform(op, 'decode')
          }}
        >
          <SelectTrigger size="sm" className="w-full">
            <SelectValue placeholder={t('decode_placeholder')} />
          </SelectTrigger>
          <SelectContent>
            {reversibleOps.map((op) => (
              <SelectItem key={op.key} value={op.key}>
                {op.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value=""
          disabled={disabled || !panel.content}
          onValueChange={(v) => {
            const op = reversibleOps.find((o) => o.key === v)
            if (op) onTransform(op, 'encode')
          }}
        >
          <SelectTrigger size="sm" className="w-full">
            <SelectValue placeholder={t('encode_placeholder')} />
          </SelectTrigger>
          <SelectContent>
            {reversibleOps.map((op) => (
              <SelectItem key={op.key} value={op.key}>
                {op.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value=""
          disabled={disabled || !panel.content}
          onValueChange={(v) => {
            const op = hashOps.find((o) => o.key === v)
            if (op) onTransform(op, 'encode')
          }}
        >
          <SelectTrigger size="sm" className="w-full">
            <SelectValue placeholder={t('hash_placeholder')} />
          </SelectTrigger>
          <SelectContent>
            {hashOps.map((op) => (
              <SelectItem key={op.key} value={op.key}>
                {op.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button
          size="sm"
          variant="secondary"
          className="w-full"
          disabled={disabled || !panel.content}
          onClick={onSmart}
        >
          {busy ? <Loader2 className="animate-spin" /> : <Sparkles />}
          {t('smart_decode')}
        </Button>
      </div>
    </div>
  )
}
