import { Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function SubdomainFinderGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Combines two techniques real subdomain enumeration tools use: a passive lookup that
          never touches the target, and an active DNS brute-force that does.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="crt.sh (passive, on by default)">
          Certificate Transparency logs record every publicly-trusted TLS certificate ever issued.
          Any subdomain that's ever had a certificate (via Let's Encrypt, a wildcard cert, etc.)
          shows up here — found without a single request to the target itself.
        </Step>

        <Step n={2} title="DNS brute-force (optional)">
          Tries common prefixes — <span className="font-mono">www</span>,{' '}
          <span className="font-mono">mail</span>, <span className="font-mono">api</span>,{' '}
          <span className="font-mono">staging</span>, etc. — against the domain and resolves each
          one. The wordlist is ranked by real-world frequency (SecLists), so "Top 100" already
          catches the common ones; "Top 1000+" digs deeper at the cost of more DNS traffic and
          time.
        </Step>

        <Step n={3} title="Pick a speed">
          Controls the DNS brute-force step: <b className="text-foreground">Fast</b> resolves many
          names at once with a short timeout; <b className="text-foreground">Cautious</b> spaces
          it out with fewer parallel lookups and a longer timeout — easier on nameservers that
          rate-limit. <b className="text-foreground">Normal</b> is a balanced default.
        </Step>

        <Step n={4} title='Click "Start scan" and watch it stream in'>
          Both sources merge into one candidate list, resolved concurrently. Matches appear in the
          table live as DNS confirms them. "Stop" cancels a running scan at any point.
        </Step>

        <Step n={5} title="Reading the table">
          <b className="text-foreground">Resolved</b> means the name currently has a DNS record —
          it's live right now. <b className="text-foreground">Source</b> shows how it was found:{' '}
          <span className="font-mono">crt.sh</span> (certificate history),{' '}
          <span className="font-mono">brute-force</span> (wordlist + DNS), or both. Toggle "Show
          unresolved attempts" to see brute-force misses too — off by default since most of a large
          wordlist won't exist.
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          crt.sh lookups are passive, but DNS brute-force sends real queries toward the domain's
          nameservers. Only run this against domains you're authorized to test.
        </p>
      </div>
    </div>
  )
}
