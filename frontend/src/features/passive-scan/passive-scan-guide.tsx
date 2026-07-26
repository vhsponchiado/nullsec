import { ShieldAlert, AlertTriangle, Info } from 'lucide-react'

export function PassiveScanGuide() {
  return (
    <div className="flex flex-col gap-6 text-sm">
      <div>
        <h2 className="font-semibold mb-1">How to use</h2>
        <p className="text-xs text-muted-foreground">
          Unlike every other scanner here, this one makes zero requests — it reads what's already
          in Proxy History (from browsing through the proxy, from a Spider crawl, from Repeater
          sends) and flags patterns worth a second look. Click "Scan" any time new traffic comes
          in to re-check it.
        </p>
      </div>

      <div>
        <h2 className="font-semibold mb-2">What it checks</h2>
        <div className="flex flex-col gap-3 text-xs text-muted-foreground">
          <p>
            <b className="text-foreground">Missing security headers</b> — CSP, HSTS (on HTTPS),
            X-Content-Type-Options, X-Frame-Options/frame-ancestors, Referrer-Policy. None of
            these alone is a vulnerability, but their absence removes a layer of defense.
          </p>
          <p>
            <b className="text-foreground">Cookie flags</b> — any <span className="font-mono">
              Set-Cookie
            </span>{' '}
            missing <span className="font-mono">HttpOnly</span>,{' '}
            <span className="font-mono">Secure</span>, or{' '}
            <span className="font-mono">SameSite</span>.
          </p>
          <p>
            <b className="text-foreground">Sensitive data in the URL</b> — parameter names like{' '}
            <span className="font-mono">password</span>, <span className="font-mono">token</span>
            , <span className="font-mono">session</span> showing up in the query string, where
            they get logged and leaked via Referer.
          </p>
          <p>
            <b className="text-foreground">Server version disclosure</b> — a{' '}
            <span className="font-mono">Server</span> or{' '}
            <span className="font-mono">X-Powered-By</span> header with a version number, handing
            an attacker exactly what to target.
          </p>
          <p>
            <b className="text-foreground">Permissive CORS</b> — a wildcard{' '}
            <span className="font-mono">Access-Control-Allow-Origin: *</span>, especially paired
            with <span className="font-mono">Access-Control-Allow-Credentials: true</span>.
          </p>
          <p>
            <b className="text-foreground">Credentials over plain HTTP</b> — a request that looks
            like it carries a password, sent unencrypted.
          </p>
        </div>
      </div>

      <div>
        <h2 className="font-semibold mb-2">What each icon means</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <ShieldAlert className="size-4 text-destructive shrink-0" />
            <span className="text-xs">
              <b>Critical</b> — a concrete, likely-exploitable issue (cleartext credentials, CORS
              that grants cross-origin credentialed access).
            </span>
          </div>
          <div className="flex items-center gap-2">
            <AlertTriangle className="size-4 text-warning shrink-0" />
            <span className="text-xs">
              <b>Warning</b> — a real weakness worth fixing, but not exploitable on its own
              (missing header, unflagged cookie, sensitive param in a URL).
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Info className="size-4 text-muted-foreground shrink-0" />
            <span className="text-xs">
              <b>Info</b> — minor hardening or disclosure, low impact by itself.
            </span>
          </div>
        </div>
      </div>

      <p className="text-xs text-muted-foreground">
        Findings are grouped by host + issue, since the same missing header usually shows up on
        every page of a site — expand a card to see exactly which URLs it applies to.
      </p>
    </div>
  )
}
