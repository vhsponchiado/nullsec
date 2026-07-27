import { useEffect, useMemo, useRef, useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  HeadersEditor,
  headersToHeaderRows,
  type HeaderRow,
} from '@/components/headers-editor'
import { MarkButton } from '@/components/mark-button'
import {
  PayloadSetEditor,
  resolvePayloadSet,
  DEFAULT_PAYLOAD_SET_CONFIG,
  type PayloadSetConfig,
  type BuiltinWordlists,
} from './intruder-payloads'
import { IntruderResults, IntruderResultDetailView } from './intruder-results'
import { IntruderGuide } from './intruder-guide'
import { Play, Square, Loader2 } from 'lucide-react'
import { api } from '@/lib/api'
import { useProxyEvents } from '@/lib/ws'
import { textToHeaders } from '@/lib/headers'
import type {
  IntruderAttackType,
  IntruderHeaderPair,
  IntruderResultDetail,
  IntruderResultEvent,
  IntruderResultSummary,
  IntruderStatus,
} from '@/lib/types'
import type { RequestSeed } from '@/components/request-editor'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

const MAX_REQUESTS = 20000

type AttackSpeed = 'fast' | 'normal' | 'cautious'

const SPEED_PRESETS: Record<AttackSpeed, { concurrency: number; timeoutMs: number }> = {
  fast: { concurrency: 20, timeoutMs: 8000 },
  normal: { concurrency: 5, timeoutMs: 15000 },
  cautious: { concurrency: 2, timeoutMs: 20000 },
}

const MARKER_RE = /§[^§]*§/g

function countMarkers(s: string): number {
  return (s.match(MARKER_RE) || []).length
}

function countPositions(url: string, headerRows: HeaderRow[], body: string): number {
  let n = countMarkers(url)
  for (const row of headerRows) n += countMarkers(row.value)
  return n + countMarkers(body)
}

function headerRowsToIntruderHeaders(rows: HeaderRow[]): IntruderHeaderPair[] {
  return rows.filter((r) => r.key.trim()).map((r) => ({ key: r.key.trim(), value: r.value }))
}

const ATTACK_TYPES: IntruderAttackType[] = ['sniper', 'battering_ram', 'pitchfork', 'cluster_bomb']

function estimateTotal(
  attackType: IntruderAttackType,
  numPositions: number,
  sets: string[][],
): number | null {
  if (numPositions === 0) return null
  if (attackType === 'sniper' || attackType === 'battering_ram') {
    const len = sets[0]?.length ?? 0
    if (len === 0) return null
    return attackType === 'sniper' ? numPositions * len : len
  }
  if (sets.length !== numPositions || sets.some((s) => s.length === 0)) return null
  if (attackType === 'pitchfork') return Math.min(...sets.map((s) => s.length))
  return sets.reduce((acc, s) => acc * s.length, 1)
}

export type IntruderSeed = RequestSeed

interface IntruderProps {
  seed?: IntruderSeed | null
}

export function Intruder({ seed }: IntruderProps) {
  const { t } = useTranslation('intruder')
  const [method, setMethod] = useState('GET')
  const [url, setUrl] = useState('')
  const [body, setBody] = useState('')
  const [headerRows, setHeaderRows] = useState<HeaderRow[]>([{ key: '', value: '' }])
  const lastNonce = useRef<number | null>(null)

  const urlRef = useRef<HTMLInputElement>(null)
  const bodyRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (seed && seed.nonce !== lastNonce.current) {
      lastNonce.current = seed.nonce
      setMethod(seed.value.method)
      setUrl(seed.value.url)
      setBody(seed.value.body)
      setHeaderRows(headersToHeaderRows(textToHeaders(seed.value.headersText)))
    }
  }, [seed])

  const [attackType, setAttackType] = useState<IntruderAttackType>('sniper')
  const [payloadConfigs, setPayloadConfigs] = useState<PayloadSetConfig[]>([
    { ...DEFAULT_PAYLOAD_SET_CONFIG },
  ])
  const [grep, setGrep] = useState('')
  const [speed, setSpeed] = useState<AttackSpeed>('normal')
  // Empty string means "no cap": only Concurrency/Speed governs pace. Kept
  // as a string (not number) so the field can be blank instead of forcing 0.
  const [rateLimit, setRateLimit] = useState('')
  const [wordlists, setWordlists] = useState<BuiltinWordlists>({
    passwords: [],
    usernames: [],
    credentialUsernames: [],
    credentialPasswords: [],
  })

  useEffect(() => {
    Promise.all([
      api.listPasswordWordlist(),
      api.listUsernameWordlist(),
      api.listCredentialWordlist(),
    ])
      .then(([passwords, usernames, credentials]) => {
        setWordlists({
          passwords,
          usernames,
          credentialUsernames: credentials.usernames,
          credentialPasswords: credentials.passwords,
        })
      })
      .catch(() => {
        // wordlist source becomes empty lists; other payload sources are unaffected
      })
  }, [])

  const numPositions = countPositions(url, headerRows, body)
  const neededSets =
    attackType === 'sniper' || attackType === 'battering_ram' ? (numPositions > 0 ? 1 : 0) : numPositions

  useEffect(() => {
    setPayloadConfigs((prev) => {
      if (prev.length === neededSets) return prev
      if (prev.length > neededSets) return prev.slice(0, neededSets)
      return [
        ...prev,
        ...Array.from({ length: neededSets - prev.length }, () => ({ ...DEFAULT_PAYLOAD_SET_CONFIG })),
      ]
    })
  }, [neededSets])

  const resolvedSets = useMemo(
    () => payloadConfigs.map((cfg) => resolvePayloadSet(cfg, wordlists)),
    [payloadConfigs, wordlists],
  )
  const estimate = estimateTotal(attackType, numPositions, resolvedSets)
  const overCap = estimate !== null && estimate > MAX_REQUESTS

  const [attackId, setAttackId] = useState<string | null>(null)
  const [status, setStatus] = useState<IntruderStatus | null>(null)
  const [results, setResults] = useState<IntruderResultSummary[]>([])
  const [startError, setStartError] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)

  const [selectedIndex, setSelectedIndex] = useState<number | null>(null)
  const [selectedDetail, setSelectedDetail] = useState<IntruderResultDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  // A fast/local target can finish (and broadcast results over the WS) before
  // the POST /start response even comes back, so gating on `attackId` state
  // would lose results to a render-timing race. A ref is updated the instant
  // we know the id, with no render in between, closing that window.
  const attackIdRef = useRef<string | null>(null)

  function mergeResults(incoming: IntruderResultSummary[] | null | undefined) {
    setResults((prev) => {
      const known = new Set(prev.map((r) => r.index))
      const fresh = (incoming ?? []).filter((r) => !known.has(r.index))
      return fresh.length ? [...prev, ...fresh] : prev
    })
  }

  useProxyEvents((event) => {
    if (event.type === 'intruder_result') {
      const payload = event.payload as IntruderResultEvent
      if (payload.attack_id !== attackIdRef.current) return
      mergeResults([payload.result])
    } else if (event.type === 'intruder_done' || event.type === 'intruder_stopped') {
      const payload = event.payload as { attack_id: string }
      if (payload.attack_id !== attackIdRef.current) return
      setStatus(event.type === 'intruder_done' ? 'done' : 'stopped')
    }
  })

  async function handleStart() {
    if (numPositions === 0) {
      setStartError(t('start_error_no_position'))
      return
    }
    setStarting(true)
    setStartError(null)
    setResults([])
    setSelectedIndex(null)
    setSelectedDetail(null)
    setStatus(null)
    try {
      const resp = await api.startIntruderAttack({
        template: {
          method,
          url,
          headers: headerRowsToIntruderHeaders(headerRows),
          body,
        },
        attack_type: attackType,
        payload_sets: resolvedSets,
        grep: grep || undefined,
        concurrency: SPEED_PRESETS[speed].concurrency,
        timeout_ms: SPEED_PRESETS[speed].timeoutMs,
        rate_per_sec: rateLimit.trim() ? Number(rateLimit) : undefined,
      })
      attackIdRef.current = resp.attack_id
      setAttackId(resp.attack_id)
      setStatus('running')
      // Any results generated between the attack starting and this response
      // arriving already landed server-side, so pull them in directly instead
      // of relying solely on the WS stream to have caught them all.
      try {
        const snapshot = await api.getIntruderAttack(resp.attack_id)
        mergeResults(snapshot.results)
        setStatus(snapshot.status)
      } catch {
        // WS updates will still arrive for anything after this point
      }
    } catch (err) {
      setStartError((err as Error).message)
    } finally {
      setStarting(false)
    }
  }

  async function handleStop() {
    if (!attackId) return
    try {
      await api.stopIntruderAttack(attackId)
    } catch {
      // the attack may have already finished server-side, so there is nothing to do
    }
  }

  async function handleSelectRow(index: number) {
    setSelectedIndex(index)
    setSelectedDetail(null)
    if (!attackId) return
    setDetailLoading(true)
    try {
      const detail = await api.getIntruderResult(attackId, index)
      setSelectedDetail(detail)
    } catch {
      // detail pane just stays empty
    } finally {
      setDetailLoading(false)
    }
  }

  const positionLabels = Array.from({ length: numPositions }, (_, i) => `Payload ${i + 1}`)
  const running = status === 'running'

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full min-w-0">
      <div className="min-w-0 flex flex-col gap-6 overflow-auto pr-1">
      <div className="flex flex-col gap-4 border rounded-lg p-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-muted-foreground">{t('request_section_title')}</h3>
          <p className="text-xs text-muted-foreground">
            <Trans
              i18nKey="request_mark_hint"
              ns="intruder"
              values={{ marker: '§' }}
              components={[<span className="font-mono" key="0" />]}
            />
          </p>
        </div>

        <div className="flex gap-2 items-end">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="intruder-method">{t('method_label')}</Label>
            <Select value={method} onValueChange={setMethod} disabled={running}>
              <SelectTrigger id="intruder-method" size="lg" className="w-28">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {METHODS.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5 flex-1">
            <Label htmlFor="intruder-url">{t('url_label')}</Label>
            <div className="flex gap-2">
              <Input
                id="intruder-url"
                ref={urlRef}
                size="lg"
                className="font-mono"
                value={url}
                disabled={running}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t('url_placeholder')}
              />
              <MarkButton
                getEl={() => urlRef.current}
                value={url}
                onChange={setUrl}
                size="icon-lg"
                className="shrink-0"
              />
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="intruder-body">{t('body_label')}</Label>
          <div className="flex gap-2">
            <Textarea
              id="intruder-body"
              ref={bodyRef}
              className="font-mono text-sm min-h-24 resize-y flex-1"
              value={body}
              disabled={running}
              onChange={(e) => setBody(e.target.value)}
              placeholder={t('body_placeholder')}
            />
            <MarkButton getEl={() => bodyRef.current} value={body} onChange={setBody} className="shrink-0" />
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label>{t('headers_label')}</Label>
          <HeadersEditor rows={headerRows} onChange={setHeaderRows} markable />
        </div>

        <div className="flex items-center gap-2">
          <Badge variant="secondary">{t('positions_marked', { count: numPositions })}</Badge>
        </div>
      </div>

      <div className="flex flex-col gap-4 border rounded-lg p-4">
        <h3 className="text-sm font-medium text-muted-foreground">{t('attack_section_title')}</h3>

        <div className="flex gap-3 items-end flex-wrap">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="intruder-attack-type">{t('attack_type_label')}</Label>
            <Select
              value={attackType}
              onValueChange={(v) => setAttackType(v as IntruderAttackType)}
              disabled={running}
            >
              <SelectTrigger id="intruder-attack-type" size="lg" className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ATTACK_TYPES.map((at) => (
                  <SelectItem key={at} value={at}>
                    {t(`attack_${at}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5 flex-1 min-w-48">
            <Label htmlFor="intruder-grep">{t('grep_label')}</Label>
            <Input
              id="intruder-grep"
              size="lg"
              className="font-mono"
              value={grep}
              disabled={running}
              onChange={(e) => setGrep(e.target.value)}
              placeholder={t('grep_placeholder')}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="intruder-speed">{t('speed_label')}</Label>
            <Select value={speed} onValueChange={(v) => setSpeed(v as AttackSpeed)} disabled={running}>
              <SelectTrigger id="intruder-speed" size="lg" className="w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(SPEED_PRESETS) as AttackSpeed[]).map((s) => (
                  <SelectItem key={s} value={s}>
                    {t(`speed_${s}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="intruder-rate-limit">{t('rate_limit_label')}</Label>
            <Input
              id="intruder-rate-limit"
              size="lg"
              type="number"
              min={1}
              max={100}
              className="w-28 font-mono"
              value={rateLimit}
              disabled={running}
              onChange={(e) => setRateLimit(e.target.value)}
              placeholder={t('rate_limit_placeholder')}
            />
          </div>
        </div>
        <p className="text-xs text-muted-foreground">{t(`attack_desc_${attackType}`)}</p>
        <p className="text-xs text-muted-foreground">
          {speed === 'fast' &&
            t('speed_hint_fast', {
              concurrency: SPEED_PRESETS.fast.concurrency,
              timeout: SPEED_PRESETS.fast.timeoutMs / 1000,
            })}
          {speed === 'normal' &&
            t('speed_hint_normal', {
              concurrency: SPEED_PRESETS.normal.concurrency,
              timeout: SPEED_PRESETS.normal.timeoutMs / 1000,
            })}
          {speed === 'cautious' &&
            t('speed_hint_cautious', {
              concurrency: SPEED_PRESETS.cautious.concurrency,
              timeout: SPEED_PRESETS.cautious.timeoutMs / 1000,
            })}
          {rateLimit.trim() && t('rate_limit_hint', { rate: rateLimit })}
        </p>

        {numPositions === 0 ? (
          <p className="text-sm text-muted-foreground">{t('no_positions_marked')}</p>
        ) : (
          <div className="flex flex-col gap-3">
            {payloadConfigs.map((cfg, i) => (
              <PayloadSetEditor
                key={i}
                label={
                  attackType === 'sniper' || attackType === 'battering_ram'
                    ? t('payload_set_single')
                    : t('payload_set_numbered', { index: i + 1, label: positionLabels[i] })
                }
                config={cfg}
                wordlists={wordlists}
                onChange={(next) =>
                  setPayloadConfigs((prev) => prev.map((c, idx) => (idx === i ? next : c)))
                }
              />
            ))}
          </div>
        )}

        <Separator />

        <div className="flex items-center gap-3">
          {!running ? (
            <Button size="lg" onClick={handleStart} disabled={starting || numPositions === 0 || overCap}>
              {starting ? <Loader2 className="animate-spin" /> : <Play />}
              {t('start_attack')}
            </Button>
          ) : (
            <Button size="lg" variant="destructive" onClick={handleStop}>
              <Square />
              {t('stop')}
            </Button>
          )}

          {estimate !== null && (
            <Badge variant={overCap ? 'outline' : 'secondary'} className={overCap ? 'border-destructive text-destructive' : ''}>
              {t('estimate_requests', { count: estimate })}
              {overCap && t('estimate_over_cap', { limit: MAX_REQUESTS.toLocaleString() })}
            </Badge>
          )}

          {status && (
            <Badge variant="outline">
              {status === 'running' && <Loader2 className="size-3 animate-spin" />}
              {status}
              {results.length > 0 &&
                (estimate
                  ? t('attack_status_done_of_estimate', { done: results.length, estimate })
                  : t('attack_status_done', { done: results.length }))}
            </Badge>
          )}
        </div>

        {startError && <div className="text-sm text-destructive">{startError}</div>}
      </div>

      <div className="flex flex-col gap-3">
        <h3 className="text-sm font-medium text-muted-foreground">{t('results_title')}</h3>
        <IntruderResults
          results={results}
          positionLabels={positionLabels}
          selectedIndex={selectedIndex}
          onSelectRow={handleSelectRow}
          hasGrep={grep.trim().length > 0}
        />
      </div>

      <div className="border rounded-lg min-w-0">
        <div className="px-4 py-2.5 border-b bg-muted/40 text-sm font-medium">{t('detail_title')}</div>
        <IntruderResultDetailView detail={selectedDetail} loading={detailLoading} />
      </div>
      </div>

      <aside className="min-w-0 border-l pl-6 overflow-auto">
        <IntruderGuide />
      </aside>
    </div>
  )
}
