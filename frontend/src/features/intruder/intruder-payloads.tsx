import { useId, useRef, useState } from 'react'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ChevronDown, ChevronUp, Upload } from 'lucide-react'

export type PayloadSource = 'simple' | 'wordlist' | 'numbers'

/** Which built-in list to draw from when source === 'wordlist'. The
 * credential-* pair select the username/password half of the same
 * username:password list, in matching order — pick one for each of two
 * marked positions and run a Pitchfork attack to test the real pairs,
 * instead of Cluster bomb's every-combination behavior. */
export type BuiltinWordlist =
  | 'passwords'
  | 'usernames'
  | 'credential-usernames'
  | 'credential-passwords'

export interface BuiltinWordlists {
  passwords: string[]
  usernames: string[]
  credentialUsernames: string[]
  credentialPasswords: string[]
}

export interface PayloadSetConfig {
  source: PayloadSource
  simpleList: string
  wordlist: BuiltinWordlist
  numberFrom: string
  numberTo: string
  numberStep: string
  zeroPad: boolean
}

export const DEFAULT_PAYLOAD_SET_CONFIG: PayloadSetConfig = {
  source: 'simple',
  simpleList: '',
  wordlist: 'passwords',
  numberFrom: '1',
  numberTo: '10',
  numberStep: '1',
  zeroPad: false,
}

const SOURCE_LABELS: Record<PayloadSource, string> = {
  simple: 'Custom list (paste or upload .txt)',
  wordlist: 'Built-in wordlist',
  numbers: 'Number range',
}

const WORDLIST_LABELS: Record<BuiltinWordlist, string> = {
  passwords: 'Passwords',
  usernames: 'Usernames',
  'credential-usernames': 'Credentials — usernames',
  'credential-passwords': 'Credentials — passwords',
}

function wordlistFor(source: BuiltinWordlist, wordlists: BuiltinWordlists): string[] {
  switch (source) {
    case 'passwords':
      return wordlists.passwords
    case 'usernames':
      return wordlists.usernames
    case 'credential-usernames':
      return wordlists.credentialUsernames
    case 'credential-passwords':
      return wordlists.credentialPasswords
  }
}

function numberRange(cfg: PayloadSetConfig): string[] {
  const from = parseInt(cfg.numberFrom, 10)
  const to = parseInt(cfg.numberTo, 10)
  const step = Math.max(1, parseInt(cfg.numberStep, 10) || 1)
  if (Number.isNaN(from) || Number.isNaN(to) || to < from) return []
  const width = String(to).length
  const out: string[] = []
  for (let n = from; n <= to; n += step) {
    out.push(cfg.zeroPad ? String(n).padStart(width, '0') : String(n))
  }
  return out
}

export function resolvePayloadSet(cfg: PayloadSetConfig, wordlists: BuiltinWordlists): string[] {
  switch (cfg.source) {
    case 'simple':
      return cfg.simpleList.split('\n').filter((line) => line.length > 0)
    case 'wordlist':
      return wordlistFor(cfg.wordlist, wordlists)
    case 'numbers':
      return numberRange(cfg)
  }
}

interface PayloadSetEditorProps {
  label: string
  config: PayloadSetConfig
  onChange: (config: PayloadSetConfig) => void
  wordlists: BuiltinWordlists
}

export function PayloadSetEditor({ label, config, onChange, wordlists }: PayloadSetEditorProps) {
  const [previewOpen, setPreviewOpen] = useState(false)
  const [loadedFile, setLoadedFile] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const count = resolvePayloadSet(config, wordlists).length
  // One PayloadSetEditor renders per marked position, so ids need to be
  // unique per instance — a hardcoded string would duplicate across
  // multiple positions and break every Label's htmlFor association.
  const uid = useId()

  async function handleFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = '' // allow re-selecting the same file later
    if (!file) return
    const text = await file.text()
    onChange({ ...config, simpleList: text.replace(/\r\n/g, '\n') })
    setLoadedFile(file.name)
  }

  return (
    <div className="border rounded-lg p-4 flex flex-col gap-3 min-w-0">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">{label}</h4>
        <Badge variant="secondary">{count} payload{count === 1 ? '' : 's'}</Badge>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${uid}-source`} className="text-xs">Source</Label>
        <Select
          value={config.source}
          onValueChange={(v) => onChange({ ...config, source: v as PayloadSource })}
        >
          <SelectTrigger id={`${uid}-source`} size="lg" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(Object.keys(SOURCE_LABELS) as PayloadSource[]).map((s) => (
              <SelectItem key={s} value={s}>
                {SOURCE_LABELS[s]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {config.source === 'simple' && (
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <Label htmlFor={`${uid}-simple-list`} className="text-xs">One value per line</Label>
            <div className="flex items-center gap-2">
              {loadedFile && (
                <span className="text-xs text-muted-foreground">Loaded {loadedFile}</span>
              )}
              <Button size="sm" variant="outline" onClick={() => fileInputRef.current?.click()}>
                <Upload />
                Upload .txt
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".txt,text/plain"
                className="hidden"
                onChange={handleFileSelected}
              />
            </div>
          </div>
          <Textarea
            id={`${uid}-simple-list`}
            className="font-mono text-sm min-h-32 resize-y"
            value={config.simpleList}
            onChange={(e) => {
              setLoadedFile(null)
              onChange({ ...config, simpleList: e.target.value })
            }}
            placeholder={'admin\ntest\nroot\n…paste one value per line, or upload a .txt file above'}
          />
        </div>
      )}

      {config.source === 'wordlist' && (
        <div className="flex flex-col gap-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${uid}-wordlist`} className="text-xs">Wordlist</Label>
            <Select
              value={config.wordlist}
              onValueChange={(v) => onChange({ ...config, wordlist: v as BuiltinWordlist })}
            >
              <SelectTrigger id={`${uid}-wordlist`} size="lg" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(WORDLIST_LABELS) as BuiltinWordlist[]).map((w) => (
                  <SelectItem key={w} value={w}>
                    {WORDLIST_LABELS[w]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {(() => {
            const list = wordlistFor(config.wordlist, wordlists)
            return (
              <>
                <p className="text-xs text-muted-foreground">
                  {list.length} {list.length === 1 ? 'entry' : 'entries'} in this wordlist.
                  {(config.wordlist === 'credential-usernames' ||
                    config.wordlist === 'credential-passwords') &&
                    ' Pair with the matching credential list via a Pitchfork attack to test real username:password combos.'}
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  className="w-fit"
                  onClick={() => setPreviewOpen((v) => !v)}
                >
                  {previewOpen ? <ChevronUp /> : <ChevronDown />}
                  {previewOpen ? 'Hide preview' : 'Preview'}
                </Button>
                {previewOpen && (
                  <div className="border rounded-md max-h-40 overflow-auto p-2 font-mono text-xs bg-muted/20">
                    {list.slice(0, 40).join('\n')}
                    {list.length > 40 && `\n… +${list.length - 40} more`}
                  </div>
                )}
              </>
            )
          })()}
        </div>
      )}

      {config.source === 'numbers' && (
        <div className="flex gap-3 items-end flex-wrap">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${uid}-number-from`} className="text-xs">From</Label>
            <Input
              id={`${uid}-number-from`}
              size="lg"
              className="w-24 font-mono"
              value={config.numberFrom}
              onChange={(e) => onChange({ ...config, numberFrom: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${uid}-number-to`} className="text-xs">To</Label>
            <Input
              id={`${uid}-number-to`}
              size="lg"
              className="w-24 font-mono"
              value={config.numberTo}
              onChange={(e) => onChange({ ...config, numberTo: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${uid}-number-step`} className="text-xs">Step</Label>
            <Input
              id={`${uid}-number-step`}
              size="lg"
              className="w-20 font-mono"
              value={config.numberStep}
              onChange={(e) => onChange({ ...config, numberStep: e.target.value })}
            />
          </div>
          <div className="flex items-center gap-2 pb-1.5">
            <Switch
              id={`${uid}-zero-pad`}
              checked={config.zeroPad}
              onCheckedChange={(v) => onChange({ ...config, zeroPad: v })}
            />
            <Label htmlFor={`${uid}-zero-pad`} className="text-xs">Zero-pad</Label>
          </div>
        </div>
      )}
    </div>
  )
}
