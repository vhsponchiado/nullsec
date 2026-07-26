import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ShieldCheck, Download, RefreshCw } from 'lucide-react'
import { api, caCertUrl } from '@/lib/api'

export function Certificate() {
  const [regenerating, setRegenerating] = useState(false)

  async function handleRegenerate() {
    if (
      !confirm(
        'Regenerating the CA invalidates every certificate issued so far. Already-visited HTTPS sites will need a fresh handshake. Continue?',
      )
    )
      return
    setRegenerating(true)
    try {
      await api.regenerateCA()
    } finally {
      setRegenerating(false)
    }
  }

  return (
    <div className="max-w-2xl flex flex-col gap-6">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <ShieldCheck size={20} className="text-primary" />
            <CardTitle>Nullsec CA</CardTitle>
          </div>
          <CardDescription>
            Root certificate used to intercept HTTPS traffic. Install it in the browser or system
            you'll use for testing so you can browse without certificate warnings.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex gap-3">
          <Button size="lg" asChild>
            <a href={caCertUrl} download="nullsec-ca-cert.pem">
              <Download />
              Download certificate
            </a>
          </Button>
          <Button size="lg" variant="outline" onClick={handleRegenerate} disabled={regenerating}>
            <RefreshCw className={regenerating ? 'animate-spin' : ''} />
            Regenerate CA
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Automatic install</CardTitle>
          <CardDescription>
            Via the backend's command line, using the same <code>smallstep/truststore</code> Hetty
            uses:
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted rounded-md p-4 text-sm font-mono overflow-x-auto">
            go run ./cmd/server cert install
          </pre>
          <p className="text-sm text-muted-foreground mt-3">
            Installs into the system trust store — may prompt for your password. To undo it, use{' '}
            <code>cert uninstall</code>.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>No-install alternative</CardTitle>
          <CardDescription>
            Opens a throwaway Chrome profile already configured with the proxy, ignoring
            certificate warnings — same technique Burp Suite and Hetty (<code>--chrome</code>)
            use. Nothing is saved between runs, and your real Chrome profile is never touched.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted rounded-md p-4 text-sm font-mono overflow-x-auto">
            go run ./cmd/server chrome
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
