import { ShieldAlert, CheckCircle2, Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function XssGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Two ways to test: automatic (the scanner does everything) or manual (you pick the
          payload and test it by hand).
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="Pick a target with a parameter">
          Paste a URL that reflects input somewhere in the page, like{' '}
          <code className="font-mono">?q=test</code> or <code className="font-mono">?name=x</code>
          . If the endpoint only accepts POST, fill in the Body field too (format{' '}
          <code className="font-mono">key=value&other=value</code>).
        </Step>

        <Step n={2} title='Automatic mode: click "Scan"'>
          Swaps each parameter's value, one at a time, through 13 payloads covering the common
          reflection contexts (raw HTML, inside an attribute, inside an inline script, a{' '}
          <span className="font-mono">javascript:</span> URL) — then checks whether each one
          survives <b className="text-foreground">unescaped</b> in the response.
        </Step>

        <Step n={3} title='Manual mode: click "Use" on the wordlist'>
          Copies the payload to the clipboard. Paste it directly into the URL (in place of the
          parameter value), into the Body, or send it and read the response yourself.
        </Step>

        <Step n={4} title="Read the results per parameter">
          Only payloads that reflected back show up in a parameter's card — a payload that never
          appears in the response at all is hidden (still counted in the card's footer).
        </Step>
      </div>

      <div>
        <h2 className="font-semibold mb-2">What each icon means</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <ShieldAlert className="size-4 text-destructive shrink-0" />
            <span className="text-xs">
              <b>Critical</b> — the exact payload (special characters intact) showed up in the
              response. The input isn't being sanitized here.
            </span>
          </div>
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />
            <span className="text-xs">
              <b>Info</b> — reflected, but HTML-escaped (e.g. <code>&amp;lt;script&amp;gt;</code>)
              — looks sanitized here.
            </span>
          </div>
        </div>
      </div>

      <div>
        <h2 className="font-semibold mb-2">Reflection contexts</h2>
        <div className="flex flex-col gap-1.5 text-xs text-muted-foreground">
          <p>
            <b className="text-foreground">HTML tag</b> — the payload becomes a brand new tag
            (<code className="font-mono">&lt;script&gt;</code>,{' '}
            <code className="font-mono">&lt;svg onload=…&gt;</code>) — works when input lands
            directly in the page body.
          </p>
          <p>
            <b className="text-foreground">Attribute break-out</b> — closes a quote/attribute
            first (<code className="font-mono">"&gt;</code>) — works when input lands inside an
            existing tag's attribute.
          </p>
          <p>
            <b className="text-foreground">Inline script break-out</b> — closes a JS string
            literal inside an existing <code className="font-mono">&lt;script&gt;</code> block.
          </p>
          <p>
            <b className="text-foreground">URL / href</b> — a <code className="font-mono">
              javascript:
            </code>{' '}
            scheme, for when input becomes a link's <code className="font-mono">href</code>.
          </p>
        </div>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          This is a reflected-XSS signal check — it doesn't execute JavaScript or check
          DOM-based/stored XSS. A "critical" finding here still deserves manual confirmation in a
          browser before reporting it. Only use it on targets you have explicit authorization to
          test.
        </p>
      </div>
    </div>
  )
}
