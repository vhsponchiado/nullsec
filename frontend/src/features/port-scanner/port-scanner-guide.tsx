import { Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function PortScannerGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          An nmap-style TCP connect scanner: it attempts a real handshake on each port and reports
          what came back. No raw sockets / SYN scan — that needs root — so this is the equivalent
          of <span className="font-mono">nmap -sT</span>.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="Enter a host">
          A hostname or IP. <span className="font-mono">scanme.nmap.org</span> is the Nmap
          project's own public test target if you want to try it out safely.
        </Step>

        <Step n={2} title="Pick a port range">
          <b className="text-foreground">Top 100 / Top 1000</b> scan the most commonly open TCP
          ports (ranked by real-world frequency, from nmap's own service database) — fast and
          usually enough. <b className="text-foreground">Custom range</b> takes a spec like{' '}
          <span className="font-mono">22,80,443,8000-8100</span> for anything else, up to the full
          1-65535 range.
        </Step>

        <Step n={3} title="Pick a speed">
          <b className="text-foreground">Fast</b> pushes concurrency up and the timeout down —
          quickest, but can misread a slow/rate-limited target as filtered.{' '}
          <b className="text-foreground">Cautious</b> does the opposite — fewer parallel
          connections, longer timeout, easier on a target that rate-limits or has a sensitive
          firewall. <b className="text-foreground">Normal</b> is a balanced default.
        </Step>

        <Step n={4} title='Click "Start scan" and watch it stream in'>
          Every port is probed concurrently, and results land in the table as they come back.
          "Stop" cancels a running scan at any point — handy for a big custom range you want to
          cut short.
        </Step>

        <Step n={5} title="Reading the state column">
          <b className="text-foreground">open</b> — the handshake succeeded; a service is
          listening. <b className="text-foreground">closed</b> — the host answered but refused the
          connection; nothing's there. <b className="text-foreground">filtered</b> — no response
          at all before the timeout, usually a firewall silently dropping the packets.
        </Step>

        <Step n={6} title="Service and Banner columns">
          <b className="text-foreground">Service</b> is the port's well-known name (e.g. 22 →
          ssh), not a live detection. <b className="text-foreground">Banner</b> is whatever bytes
          the open port sent back unprompted in the first moment after connecting — many services
          (SSH, FTP, SMTP) announce themselves this way; others (plain HTTP) stay silent until
          asked, so it's often empty.
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          This makes real TCP connections to every port scanned. Only use it against hosts you're
          authorized to test.
        </p>
      </div>
    </div>
  )
}
