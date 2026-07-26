import { Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function DecoderGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Works like Burp's Decoder: each row is a panel holding some text. Transform it and the
          result appears as a new panel below, so you can keep chaining — decode base64, then URL,
          then hash it, all in one stack.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="Paste text into the first panel">
          The top panel is your input — type or paste anything into it.
        </Step>

        <Step n={2} title="Pick a transform from a panel's dropdowns">
          <span className="font-mono">Decode as…</span> and{' '}
          <span className="font-mono">Encode as…</span> cover Base64 (standard and URL-safe),
          Base32, Hex, URL percent-encoding, HTML entities and Gzip (shown as base64, since
          compressed bytes aren't printable text). <span className="font-mono">Hash…</span> runs
          MD5/SHA-1/SHA-256/SHA-512 — one-way, so there's no decode side for those.
        </Step>

        <Step n={3} title='Not sure what it is? Click "Smart decode"'>
          It detects the encoding automatically (HTML entities, %-encoding, Base64/32, hex, gzip)
          and peels off as many layers as it can find in one click, adding one panel per layer.
        </Step>

        <Step n={4} title="Edit any panel, or use Text/Hex to inspect bytes">
          Editing a panel's content drops every panel after it, since they'd otherwise be showing
          a stale result. The Text/Hex toggle switches a panel between its plain-text view and a
          hex dump of the underlying bytes — handy after a gzip decode or anything non-printable.
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          Everything here runs locally against the text you provide — no requests are made to any
          target.
        </p>
      </div>
    </div>
  )
}
