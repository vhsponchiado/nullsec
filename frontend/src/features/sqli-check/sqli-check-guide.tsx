import { ShieldAlert, AlertTriangle, CheckCircle2, Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function SqliGuide() {
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
          Paste a URL that has something to inject in the query string, like{' '}
          <code className="font-mono">?id=1</code> or <code className="font-mono">?cat=2</code>.
          If the endpoint only accepts POST, fill in the Body field too (format{' '}
          <code className="font-mono">key=value&other=value</code>). If you need a login/session,
          paste the header in <code className="font-mono">Headers</code> (e.g.{' '}
          <code className="font-mono">Authorization: Bearer ...</code> or{' '}
          <code className="font-mono">Cookie: session=...</code>).
        </Step>

        <Step n={2} title='Automatic mode: click "Scan"'>
          The scanner swaps each parameter's value, one at a time, through each of the 18
          wordlist payloads — and compares the response to a "clean" request (baseline) to spot
          differences in error, status, size, or time.
        </Step>

        <Step n={3} title='Manual mode: click "Use" on the wordlist'>
          Copies the payload to the clipboard. Paste it directly into the URL (in place of the
          parameter value), into the Body, or send it to Repeater to see the full response
          yourself.
        </Step>

        <Step n={4} title="Read the results per parameter">
          Each tested parameter becomes a card. Only payloads that produced some signal show up
          there — the "no signal" ones stay hidden (counted in the card's footer).
        </Step>
      </div>

      <div>
        <h2 className="font-semibold mb-2">What each icon means</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <ShieldAlert className="size-4 text-destructive shrink-0" />
            <span className="text-xs">
              <b>Critical</b> — database error in the response, or a confirmed time delay. Strong
              signal of a real SQLi.
            </span>
          </div>
          <div className="flex items-center gap-2">
            <AlertTriangle className="size-4 text-warning shrink-0" />
            <span className="text-xs">
              <b>Warning</b> — response status or size changed. Could be SQLi, could be
              coincidence — worth investigating by hand.
            </span>
          </div>
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 text-muted-foreground shrink-0" />
            <span className="text-xs">
              <b>Info</b> — nothing abnormal detected for this payload.
            </span>
          </div>
        </div>
      </div>

      <div>
        <h2 className="font-semibold mb-2">Payload types</h2>
        <div className="flex flex-col gap-1.5 text-xs text-muted-foreground">
          <p>
            <b className="text-foreground">Error-based</b> — tries to break the query (quotes,
            UNION SELECT) expecting a SQL syntax error to leak into the response.
          </p>
          <p>
            <b className="text-foreground">Boolean-based</b> — injects always-true/always-false
            conditions (<code className="font-mono">OR 1=1</code>) expecting the response content
            to change.
          </p>
          <p>
            <b className="text-foreground">Time-based</b> — injects a delay (
            <code className="font-mono">SLEEP(3)</code>) and measures whether the response really
            took longer — useful when there's no error or visible difference ("blind" SQLi).
          </p>
        </div>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          This makes real requests against the target (including deliberate delays). Only use it
          on targets you have explicit authorization to test.
        </p>
      </div>
    </div>
  )
}
