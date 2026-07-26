import { Info } from 'lucide-react'
import { Step } from '@/components/guide-step'

export function SpiderGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Starts at a URL and follows every same-site link it finds, mapping out the site —
          Burp/ZAP call this a "spider". Every page it fetches is saved into Proxy History live,
          exactly like intercepted traffic, ready to send to Repeater, Intruder, or the scanners.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        <Step n={1} title="Set a start URL and limits">
          <b className="text-foreground">Max depth</b> is how many link-hops away from the start
          page it'll follow. <b className="text-foreground">Max pages</b> is a hard cap on the
          total pages fetched — both exist so a crawl can't run away on a huge site. "Include
          subdomains" widens scope from just the exact host to anything under it.
        </Step>

        <Step n={2} title="Pick a speed">
          Controls how many pages it fetches at once. <b className="text-foreground">Cautious</b>{' '}
          is gentler on the target; <b className="text-foreground">Fast</b> finishes quicker at
          the cost of more concurrent load.
        </Step>

        <Step n={3} title='Click "Start crawl" and watch it stream in'>
          New pages land in the table (and in Proxy History) as they're fetched. Each row shows
          the page's title, how many hops deep it is, how many links it had, and how many forms
          were found on it. "Stop" cancels a running crawl at any point.
        </Step>

        <Step n={4} title="Forms are the interesting part">
          Every <span className="font-mono">&lt;form&gt;</span> found gets its action URL,
          method, and input field names recorded — a quick map of every place the site accepts
          input, worth feeding into the SQL Injection or XSS scanners next.
        </Step>
      </div>

      <div className="flex gap-2 items-start bg-muted/40 rounded-lg p-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" />
        <p className="text-xs text-muted-foreground">
          This deliberately ignores <span className="font-mono">robots.txt</span> — that file is
          a convention for search engines, not a security boundary, and a pentest spider exists to
          find what a polite crawler wouldn't. It does make a real request per page found, though
          — only point it at hosts you're authorized to test.
        </p>
      </div>
    </div>
  )
}
