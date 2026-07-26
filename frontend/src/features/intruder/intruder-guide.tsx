import { Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function IntruderGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Burp Intruder's idea: mark spots in a request, throw payloads at them, and compare every
          response.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="Mark payload positions">
          Select some text in the URL or Body and click{' '}
          <span className="font-mono">§</span> — it wraps the selection in{' '}
          <span className="font-mono">§…§</span> markers. Header values can be marked the same
          way. Each marked spot becomes one position, numbered in the order they appear (URL, then
          headers, then body).
        </Step>

        <Step n={2} title="Pick an attack type">
          <b className="text-foreground">Sniper</b> tests one position at a time with a single
          payload set. <b className="text-foreground">Battering ram</b> puts the same payload in
          every position at once. <b className="text-foreground">Pitchfork</b> pairs up one
          payload set per position, advancing together (e.g. matching username + password lists).{' '}
          <b className="text-foreground">Cluster bomb</b> tries every combination across one
          payload set per position — the most thorough, and the most requests.
        </Step>

        <Step n={3} title="Fill in the payload set(s)">
          Each position needing a set gets its own panel with three sources:{' '}
          <b className="text-foreground">Simple list</b> (paste values, one per line),{' '}
          <b className="text-foreground">Common passwords</b> (the built-in wordlist of 1000 real
          leaked/common passwords — click "Preview" to browse it before using it), or{' '}
          <b className="text-foreground">Number range</b> (from/to/step, with optional zero-
          padding).
        </Step>

        <Step n={4} title="Optionally set a grep-match">
          Type a string you expect to see only in a successful response (e.g.{' '}
          <span className="font-mono">Welcome back</span>). Any result containing it gets flagged
          with a checkmark in the Match column, so you can spot the hit in a sea of 403s at a
          glance.
        </Step>

        <Step n={5} title='Click "Start attack" and watch the table fill in'>
          Results stream in live as each request completes. Click a column header to sort by
          status, length or time; click a row to see the exact request that was sent and the full
          response it got back. "Stop" cancels a running attack at any point.
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          This fires real requests at the target, one per payload combination (up to 20,000 per
          attack). Only use it against hosts you're authorized to test.
        </p>
      </div>
    </div>
  )
}
